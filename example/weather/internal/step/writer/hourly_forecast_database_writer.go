package writer

import (
	"context"
	"fmt"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	appRepo "github.com/tigerroll/surfin/example/weather/internal/repository"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	batch_config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
)

// HourlyForecastDatabaseWriterConfig holds configuration specific to the HourlyForecastDatabaseWriter, typically for JSL property binding.
type HourlyForecastDatabaseWriterConfig struct {
	TargetResourceName string `yaml:"targetResourceName,omitempty"` // TargetResourceName is the name of the resource (e.g., database connection) to use.
	Database           string `yaml:"database,omitempty"`           // Database is an alias for TargetResourceName, for backward compatibility.
}

// HourlyForecastDatabaseWriter implements [port.ItemWriter] for writing weather data to a database.
type HourlyForecastDatabaseWriter struct {
	Repo appRepo.WeatherRepository // Repo is the repository instance, initialized in the [Open] method based on the database type.

	DBResolver         database.DBConnectionResolver // DBResolver is the database connection resolver, used to obtain DB connections.
	Config             *batch_config.Config          // Config is the application's global configuration.
	TargetResourceName string                        // TargetResourceName is the name of the target resource (e.g., database connection), resolved from JSL properties.
	ResourcePath       string                        // ResourcePath is the path or identifier within the target resource (e.g., table name), derived from the entity.

	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext
	// writerState holds the writer's internal state.
	writerState model.ExecutionContext
	resolver    port.ExpressionResolver
	dbResolver  database.DBConnectionResolver // dbResolver is the database connection resolver.
}

// Verify that HourlyForecastDatabaseWriter implements the [port.ItemWriter[any]] interface.
var _ port.ItemWriter[any] = (*HourlyForecastDatabaseWriter)(nil)

// NewHourlyForecastDatabaseWriter creates a new [HourlyForecastDatabaseWriter] instance.
//
// Parameters:
//
//	cfg: The application's global configuration.
//	allDBConnections: A map of all established database connections.
//	resolver: An [port.ExpressionResolver] for dynamic property resolution.
//	dbResolver: A [adapter.DBConnectionResolver] for resolving database connections.
//
//	properties: A map of JSL properties for this writer.
func NewHourlyForecastDatabaseWriter(
	cfg *batch_config.Config,
	resolver port.ExpressionResolver,
	dbResolver database.DBConnectionResolver,
	properties map[string]string,
) (*HourlyForecastDatabaseWriter, error) {

	writerCfg := &HourlyForecastDatabaseWriterConfig{}
	if err := configbinder.BindProperties(properties, writerCfg); err != nil {
		return nil, exception.NewBatchError("hourly_forecast_database_writer", "Failed to bind properties", err, false, false)
	}

	// Prioritize 'targetResourceName' specified in JSL, fallback to 'database', then default.
	resourceName := writerCfg.TargetResourceName
	if resourceName == "" {
		resourceName = writerCfg.Database
	}
	if resourceName == "" {
		resourceName = "workload" // Default value
	}

	return &HourlyForecastDatabaseWriter{
		// Repo is initialized in Open.
		// The TxManager is no longer directly held by the writer; it's created on demand
		// by the ChunkStep using the TxManagerFactory and passed to the Write method.

		DBResolver:         dbResolver,
		Config:             cfg,
		TargetResourceName: resourceName,
		ResourcePath:       weather_entity.WeatherDataToStore{}.TableName(), // ResourcePath is initialized from the entity's TableName method.

		resolver:             resolver,
		stepExecutionContext: model.NewExecutionContext(),
		writerState:          model.NewExecutionContext(),
	}, nil
}

// Open initializes the writer and restores its state from the provided [model.ExecutionContext].
//
// Parameters:
//
//	ctx: The context for the operation.
//
//	ec: The ExecutionContext to initialize the writer with.
func (w *HourlyForecastDatabaseWriter) Open(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastDatabaseWriter.Open is called.")

	conn, err := w.DBResolver.ResolveDBConnection(ctx, w.TargetResourceName)
	if err != nil {
		return fmt.Errorf("failed to resolve database connection '%s': %w", w.TargetResourceName, err)
	}

	// Select the appropriate repository implementation based on the connection type.
	dbType := conn.Type()

	switch dbType {
	case "postgres", "redshift":
		w.Repo = appRepo.NewPostgresWeatherRepository(conn, dbType) // Initialize repository
	case "mysql":
		w.Repo = appRepo.NewMySQLWeatherRepository(conn, dbType)
	case "sqlite":
		w.Repo = appRepo.NewSQLiteWeatherRepository(conn, dbType)
	default:
		return fmt.Errorf("unsupported database type '%s'", dbType)
	}

	// Set stepExecutionContext and restore internal state from EC.
	w.stepExecutionContext = ec
	return w.restoreWriterStateFromExecutionContext(ctx)
}

// Write persists a chunk of items to the database within the provided transaction.
//
// Parameters:
//
//	ctx: The context for the operation.
//	items: The list of items to be written.
func (w *HourlyForecastDatabaseWriter) Write(ctx context.Context, items []any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(items) == 0 {
		logger.Debugf("No items to write.")
		return nil
	}

	// Retrieve the transaction from the context.
	currentTx, ok := tx.TxFromContext(ctx)
	if !ok {
		// If no transaction is found in the context, return an error.
		return exception.NewBatchError("hourly_forecast_database_writer", "transaction not found in context for database write", nil, false, false)
	}

	var finalDataToStore []weather_entity.WeatherDataToStore
	for _, item := range items {
		typedItem, ok := item.(*weather_entity.WeatherDataToStore)
		if !ok {
			return exception.NewBatchError("hourly_forecast_database_writer", fmt.Sprintf("unexpected input item type: %T, expected type: *weather_entity.WeatherDataToStore", item), nil, false, true)
		}
		if typedItem != nil {
			finalDataToStore = append(finalDataToStore, *typedItem)
		}
	}

	if len(finalDataToStore) == 0 {
		logger.Debugf("No valid items to write after type conversion.")
		return nil
	}

	if w.Repo == nil {
		return exception.NewBatchError("hourly_forecast_database_writer", "WeatherRepository is not initialized (was Open called?)", nil, true, false)
	}

	// Execute database operations using the retrieved transaction.
	err := w.Repo.BulkInsertWeatherData(ctx, currentTx, finalDataToStore)
	if err != nil {
		return exception.NewBatchError("hourly_forecast_database_writer", "failed to bulk insert weather data", err, true, true)
	}

	logger.Debugf("Saved chunk of weather data items to the database. Count: %d", len(finalDataToStore))
	return nil
}

// Close releases any resources held by the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
func (w *HourlyForecastDatabaseWriter) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastDatabaseWriter.Close is called.")
	// Save internal state to the Step's ExecutionContext.
	if err := w.saveWriterStateToExecutionContext(ctx); err != nil {
		logger.Errorf("HourlyForecastDatabaseWriter.Close: failed to save internal state: %v", err)
	}
	return nil
}

// SetExecutionContext sets the ExecutionContext for the writer and restores its state.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
func (w *HourlyForecastDatabaseWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w.stepExecutionContext = ec
	return w.restoreWriterStateFromExecutionContext(ctx)
}

// GetExecutionContext retrieves the current ExecutionContext from the writer.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns: The current ExecutionContext and an error if retrieval fails.
func (w *HourlyForecastDatabaseWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastDatabaseWriter.GetExecutionContext is called.")

	// Save internal state to "writer_context" in the Step ExecutionContext.
	if err := w.saveWriterStateToExecutionContext(ctx); err != nil {
		return nil, err
	}

	return w.writerState, nil
}

// GetTargetResourceName returns the name of the target resource for this writer.
func (w *HourlyForecastDatabaseWriter) GetTargetResourceName() string {
	return w.TargetResourceName
}

// GetResourcePath returns the path or identifier within the target resource for this writer.
func (w *HourlyForecastDatabaseWriter) GetResourcePath() string {
	return w.ResourcePath
}

// restoreWriterStateFromExecutionContext extracts writer-specific state from the step's ExecutionContext.
func (w *HourlyForecastDatabaseWriter) restoreWriterStateFromExecutionContext(ctx context.Context) error {
	// Extract writer-specific context from stepExecutionContext.
	writerCtxVal, ok := w.stepExecutionContext.Get("writer_context")
	var writerCtx model.ExecutionContext
	if !ok || writerCtxVal == nil {
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
		logger.Debugf("HourlyForecastDatabaseWriter: Initialized new Writer ExecutionContext.")
	} else if rcv, isEC := writerCtxVal.(model.ExecutionContext); isEC {
		writerCtx = rcv
	} else {
		logger.Warnf("HourlyForecastDatabaseWriter: ExecutionContext 'writer_context' key has unexpected type (%T). Initializing new ExecutionContext.", writerCtxVal)
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	}

	// The writer currently holds no special internal state, but copies writerState.
	w.writerState = writerCtx.Copy()
	logger.Debugf("HourlyForecastDatabaseWriter: Internal state restored.")
	return nil
}

// saveWriterStateToExecutionContext saves the writer's internal state to the step's ExecutionContext.
func (w *HourlyForecastDatabaseWriter) saveWriterStateToExecutionContext(ctx context.Context) error {
	// Extract writer-specific context from stepExecutionContext (created if not present)
	writerCtxVal, ok := w.stepExecutionContext.Get("writer_context")
	var writerCtx model.ExecutionContext
	if !ok || writerCtxVal == nil {
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	} else if rcv, isEC := writerCtxVal.(model.ExecutionContext); isEC {
		writerCtx = rcv
	} else {
		logger.Warnf("HourlyForecastDatabaseWriter: ExecutionContext key 'writer_context' has unexpected type (%T). Initializing new ExecutionContext.", writerCtxVal)
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	}

	// Save internal state to writerCtx.
	// Currently, there is no writer-specific internal state to save to writerCtx.
	// Add it here if needed in the future.

	// Set the decision.condition for the job flow directly on the stepExecutionContext.
	// This key will be promoted to JobExecutionContext by the framework if configured in JSL.
	// IMPORTANT: Also set it on w.writerState so that GetExecutionContext returns it.
	writerCtx.Put("decision.condition", "true")
	w.writerState.Put("decision.condition", "true")

	logger.Debugf("HourlyForecastDatabaseWriter: Internal state saved to ExecutionContext.")
	return nil
}
