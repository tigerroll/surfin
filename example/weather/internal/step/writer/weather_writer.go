package writer

import (
	"context"
	"fmt"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	appRepo "github.com/tigerroll/surfin/example/weather/internal/repository"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	batch_config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
)

// WeatherWriterConfig holds configuration specific to the WeatherItemWriter, typically for JSL property binding.
type WeatherWriterConfig struct {
	TargetDBName string `yaml:"targetDBName,omitempty"` // TargetDBName is the name of the database connection to use.
	Database     string `yaml:"database,omitempty"`     // Database is an alias for TargetDBName, for backward compatibility.
}

// WeatherItemWriter implements [port.ItemWriter] for writing weather data to a database.
type WeatherItemWriter struct {
	Repo appRepo.WeatherRepository // Repo is the repository instance, initialized in the [Open] method based on the database type.

	AllDBConnections map[string]adapter.DBConnection // AllDBConnections is a map of all established database connections, keyed by their name.
	DBResolver       adapter.DBConnectionResolver    // DBResolver is the database connection resolver, used to obtain DB connections.
	Config           *batch_config.Config            // Config is the application's global configuration.
	TargetDBName     string                          // TargetDBName is the name of the target database connection, resolved from JSL properties.
	TableName        string                          // TableName is the name of the target table for this writer, derived from the entity.

	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext
	// writerState holds the writer's internal state.
	writerState model.ExecutionContext
	resolver    port.ExpressionResolver
	dbResolver  adapter.DBConnectionResolver // dbResolver is the database connection resolver.
}

// Verify that WeatherItemWriter implements the [port.ItemWriter[any]] interface.
var _ port.ItemWriter[any] = (*WeatherItemWriter)(nil)

// NewWeatherWriter creates a new [WeatherItemWriter] instance.
//
// Parameters:
//
//	cfg: The application's global configuration.
//	allDBConnections: A map of all established database connections.
//	resolver: An [port.ExpressionResolver] for dynamic property resolution.
//	dbResolver: A [adapter.DBConnectionResolver] for resolving database connections.
//
// properties is a map of JSL properties for this writer.
func NewWeatherWriter(
	cfg *batch_config.Config,
	allDBConnections map[string]adapter.DBConnection,
	resolver port.ExpressionResolver,
	dbResolver adapter.DBConnectionResolver,
	properties map[string]string,
) (*WeatherItemWriter, error) {

	writerCfg := &WeatherWriterConfig{}
	if err := configbinder.BindProperties(properties, writerCfg); err != nil {
		return nil, exception.NewBatchError("weather_writer", "Failed to bind properties", err, false, false)
	}

	// Prioritize 'targetDBName' specified in JSL, fallback to 'database', then default.
	dbName := writerCfg.TargetDBName
	if dbName == "" {
		dbName = writerCfg.Database
	}
	if dbName == "" {
		dbName = "workload" // Default value
	}

	// Check for DBConnection existence. The actual connection is used during Open.
	_, ok := allDBConnections[dbName]
	if !ok {
		return nil, fmt.Errorf("database connection '%s' not found", dbName)
	}

	return &WeatherItemWriter{
		// Repo is initialized in Open.
		// The TxManager is no longer directly held by the writer; it's created on demand
		// by the ChunkStep using the TxManagerFactory and passed to the Write method.

		AllDBConnections: allDBConnections,
		DBResolver:       dbResolver,
		Config:           cfg,
		TargetDBName:     dbName,
		TableName:        weather_entity.WeatherDataToStore{}.TableName(), // TableName is initialized from the entity's TableName method.

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
// ec is the ExecutionContext to initialize the writer with.
func (w *WeatherItemWriter) Open(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("WeatherItemWriter.Open is called.")

	conn, ok := w.AllDBConnections[w.TargetDBName]
	if !ok {
		return fmt.Errorf("database connection '%s' not found", w.TargetDBName)
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
//
// tx is the current transaction.
// items is the list of items to be written.
func (w *WeatherItemWriter) Write(ctx context.Context, tx tx.Tx, items []any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(items) == 0 {
		logger.Debugf("No items to write.")
		return nil
	}

	var finalDataToStore []weather_entity.WeatherDataToStore
	for _, item := range items {
		typedItem, ok := item.(*weather_entity.WeatherDataToStore)
		if !ok {
			return exception.NewBatchError("weather_writer", fmt.Sprintf("unexpected input item type: %T, expected type: *weather_entity.WeatherDataToStore", item), nil, false, true)
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
		return exception.NewBatchError("weather_writer", "WeatherRepository is not initialized (was Open called?)", nil, true, false)
	}

	err := w.Repo.BulkInsertWeatherData(ctx, tx, finalDataToStore)
	if err != nil {
		// Changed isRetryable to true based on common DB error handling.
		return exception.NewBatchError("weather_writer", "failed to bulk insert weather data", err, true, true)
	}

	logger.Debugf("Saved chunk of weather data items to the database. Count: %d", len(finalDataToStore))
	return nil
}

// Close releases any resources held by the writer.
//
// ctx is the context for the operation.
func (w *WeatherItemWriter) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	logger.Debugf("WeatherItemWriter.Close is called.")
	// Save internal state to the Step's ExecutionContext.
	if err := w.saveWriterStateToExecutionContext(ctx); err != nil {
		logger.Errorf("WeatherWriter.Close: failed to save internal state: %v", err)
	}
	return nil
}

// SetExecutionContext sets the ExecutionContext for the writer and restores its state.
//
// ctx is the context for the operation.
// ec is the ExecutionContext to set.
func (w *WeatherItemWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
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
// ctx is the context for the operation.
//
// Returns the current ExecutionContext and an error if retrieval fails.
func (w *WeatherItemWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	logger.Debugf("WeatherWriter.GetExecutionContext is called.")

	// Save internal state to "writer_context" in the Step ExecutionContext.
	if err := w.saveWriterStateToExecutionContext(ctx); err != nil {
		return nil, err
	}

	return w.writerState, nil
}

// GetTargetDBName returns the name of the target database for this writer.
func (w *WeatherItemWriter) GetTargetDBName() string {
	return w.TargetDBName
}

// GetTableName returns the name of the target table for this writer.
func (w *WeatherItemWriter) GetTableName() string {
	return w.TableName
}

// restoreWriterStateFromExecutionContext extracts writer-specific state from the step's ExecutionContext.
func (w *WeatherItemWriter) restoreWriterStateFromExecutionContext(ctx context.Context) error {
	// Extract writer-specific context from stepExecutionContext.
	writerCtxVal, ok := w.stepExecutionContext.Get("writer_context")
	var writerCtx model.ExecutionContext
	if !ok || writerCtxVal == nil {
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
		logger.Debugf("WeatherWriter: Initialized new Writer ExecutionContext.")
	} else if rcv, isEC := writerCtxVal.(model.ExecutionContext); isEC {
		writerCtx = rcv
	} else {
		logger.Warnf("WeatherWriter: ExecutionContext 'writer_context' key has unexpected type (%T). Initializing new ExecutionContext.", writerCtxVal)
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	}

	// The writer currently holds no special internal state, but copies writerState.
	w.writerState = writerCtx.Copy()
	logger.Debugf("WeatherWriter: Internal state restored.")
	return nil
}

// saveWriterStateToExecutionContext saves the writer's internal state to the step's ExecutionContext.
func (w *WeatherItemWriter) saveWriterStateToExecutionContext(ctx context.Context) error {
	// Extract writer-specific context from stepExecutionContext (created if not present)
	writerCtxVal, ok := w.stepExecutionContext.Get("writer_context")
	var writerCtx model.ExecutionContext
	if !ok || writerCtxVal == nil {
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	} else if rcv, isEC := writerCtxVal.(model.ExecutionContext); isEC {
		writerCtx = rcv
	} else {
		logger.Warnf("WeatherWriter: ExecutionContext key 'writer_context' has unexpected type (%T). Initializing new ExecutionContext.", writerCtxVal)
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

	logger.Debugf("WeatherWriter: Internal state saved to ExecutionContext.")
	return nil
}
