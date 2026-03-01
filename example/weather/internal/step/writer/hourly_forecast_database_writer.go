package writer

import (
	"context"
	"fmt"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/component/step/writer"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	batch_config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
)

// HourlyForecastDatabaseWriterConfig holds configuration specific to the HourlyForecastDatabaseWriter, typically for JSL property binding.
type HourlyForecastDatabaseWriterConfig struct {
	TargetResourceName string `yaml:"targetResourceName,omitempty"` // TargetResourceName is the name of the resource (e.g., database connection) to use.
	Database           string `yaml:"database,omitempty"`           // Database is an alias for TargetResourceName, for backward compatibility.
	BulkSize           int    `yaml:"bulkSize,omitempty"`           // BulkSize is the maximum number of items to process in a single database operation for SqlBulkWriter.
}

// HourlyForecastDatabaseWriter implements [port.ItemWriter] for writing weather data to a database.
type HourlyForecastDatabaseWriter struct {
	sqlBulkWriter *writer.SqlBulkWriter[weather_entity.WeatherDataToStore] // sqlBulkWriter is the internal SqlBulkWriter instance.

	DBResolver              database.DBConnectionResolver // DBResolver is the database connection resolver, used to obtain DB connections.
	Config                  *batch_config.Config          // Config is the application's global configuration.
	TargetResourceName      string                        // TargetResourceName is the name of the target resource (e.g., database connection), resolved from JSL properties.
	ResourcePath            string                        // ResourcePath is the path or identifier within the target resource (e.g., table name), derived from the entity.
	bulkSize                int                           // bulkSize is the resolved bulk size for the internal SqlBulkWriter.
	resolvedConflictColumns []string                      // resolvedConflictColumns are the conflict columns determined at Open time.
	resolvedUpdateColumns   []string                      // resolvedUpdateColumns are the update columns determined at Open time.

	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext
	// writerState holds the writer's internal state.
	writerState model.ExecutionContext
	resolver    port.ExpressionResolver
}

// Verify that HourlyForecastDatabaseWriter implements the [port.ItemWriter[any]] interface.
var _ port.ItemWriter[any] = (*HourlyForecastDatabaseWriter)(nil)

// NewHourlyForecastDatabaseWriter creates a new [HourlyForecastDatabaseWriter] instance.
// It initializes the writer with application configuration, an expression resolver,
// a database connection resolver, and binds JSL properties to its specific configuration.
//
// Parameters:
//
//	cfg: The application's global configuration.
//	resolver: An [port.ExpressionResolver] for dynamic property resolution.
//	dbResolver: A [adapter.DBConnectionResolver] for resolving database connections.
//	properties: A map of JSL properties for this writer.
//
// Returns:
//
//	A new HourlyForecastDatabaseWriter instance or an error if configuration binding or validation fails.
func NewHourlyForecastDatabaseWriter(
	cfg *batch_config.Config,
	resolver port.ExpressionResolver,
	dbResolver database.DBConnectionResolver,
	properties map[string]interface{},
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

	// Resolve bulkSize, default to 1000 if not specified or invalid
	bulkSize := writerCfg.BulkSize
	if bulkSize <= 0 {
		bulkSize = 1000 // Default bulk size
	}

	return &HourlyForecastDatabaseWriter{
		DBResolver:         dbResolver,
		Config:             cfg,
		TargetResourceName: resourceName,
		ResourcePath:       weather_entity.WeatherDataToStore{}.TableName(), // ResourcePath is initialized from the entity's TableName method.
		bulkSize:           bulkSize,

		resolver:             resolver,
		stepExecutionContext: model.NewExecutionContext(),
		writerState:          model.NewExecutionContext(),
	}, nil
}

// Open initializes the writer and restores its state from the provided [model.ExecutionContext].
// It resolves the database connection and initializes the internal SqlBulkWriter.
//
// Parameters:
//
//	ctx: The context for the operation.
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

	w.resolvedConflictColumns = []string{"time", "latitude", "longitude"} // Always these for weather data

	// Initialize SqlBulkWriter
	dbType := conn.Type()
	switch dbType {
	case "postgres", "redshift", "sqlite":
		w.resolvedUpdateColumns = []string{} // DO NOTHING for these
	case "mysql":
		w.resolvedUpdateColumns = []string{"weather_code", "temperature_2m", "collected_at"} // DO UPDATE for MySQL
	default:
		return fmt.Errorf("unsupported database type '%s'", dbType)
	}

	w.sqlBulkWriter = writer.NewSqlBulkWriter[weather_entity.WeatherDataToStore](
		w.TargetResourceName+"_sql_bulk_writer", // A more specific name for the internal writer
		w.bulkSize,
		weather_entity.WeatherDataToStore{}.TableName(),
		w.resolvedConflictColumns,
		w.resolvedUpdateColumns,
	)

	// Call Open on the internal SqlBulkWriter
	if err := w.sqlBulkWriter.Open(ctx, ec); err != nil {
		return fmt.Errorf("failed to open SqlBulkWriter: %w", err)
	}

	// Set stepExecutionContext and restore internal state from EC.
	w.stepExecutionContext = ec
	return w.restoreWriterStateFromExecutionContext(ctx)
}

// Write persists a chunk of items to the database.
// It converts generic items to WeatherDataToStore and delegates to the internal SqlBulkWriter.
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

	if w.sqlBulkWriter == nil {
		return exception.NewBatchError("hourly_forecast_database_writer", "SqlBulkWriter is not initialized (was Open called?)", nil, true, false)
	}

	err := w.sqlBulkWriter.Write(ctx, finalDataToStore)
	if err != nil {
		return exception.NewBatchError("hourly_forecast_database_writer", "failed to bulk insert weather data via SqlBulkWriter", err, true, true)
	}

	logger.Debugf("Saved chunk of weather data items to the database. Count: %d", len(finalDataToStore))
	return nil
}

// Close releases any resources held by the writer, including the internal SqlBulkWriter.
// It also saves the writer's internal state to the ExecutionContext.
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
	if err := w.saveWriterStateToExecutionContext(ctx); err != nil {
		logger.Errorf("HourlyForecastDatabaseWriter.Close: failed to save internal state: %v", err)
	}

	// Close the internal SqlBulkWriter
	if w.sqlBulkWriter != nil {
		if err := w.sqlBulkWriter.Close(ctx); err != nil {
			return fmt.Errorf("failed to close SqlBulkWriter: %w", err)
		}
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
// It saves the writer's internal state before returning the context.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The current ExecutionContext and an error if retrieval or state saving fails.
func (w *HourlyForecastDatabaseWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	logger.Debugf("HourlyForecastDatabaseWriter.GetExecutionContext is called.")

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
// It initializes or restores the writer's internal state (e.g., writerState).
func (w *HourlyForecastDatabaseWriter) restoreWriterStateFromExecutionContext(ctx context.Context) error {
	writerCtxVal, ok := w.stepExecutionContext.Get("writer_context")
	var writerCtx model.ExecutionContext
	if !ok || writerCtxVal == nil {
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	} else if rcv, isEC := writerCtxVal.(model.ExecutionContext); isEC {
		writerCtx = rcv
	} else {
		logger.Warnf("HourlyForecastDatabaseWriter: ExecutionContext 'writer_context' key has unexpected type (%T). Initializing new ExecutionContext.", writerCtxVal)
		writerCtx = model.NewExecutionContext()
		w.stepExecutionContext.Put("writer_context", writerCtx)
	}

	// The writer currently holds no special internal state, but copies writerState.
	w.writerState = writerCtx.Copy()
	return nil
}

// saveWriterStateToExecutionContext saves the writer's internal state to the step's ExecutionContext.
// This includes setting the "decision.condition" key for job flow control.
func (w *HourlyForecastDatabaseWriter) saveWriterStateToExecutionContext(ctx context.Context) error {
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

	writerCtx.Put("decision.condition", "true")
	w.writerState.Put("decision.condition", "true")

	return nil
}
