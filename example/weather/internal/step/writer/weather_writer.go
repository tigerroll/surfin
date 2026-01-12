package writer

import (
	"context"
	"fmt"

	weather_entity "surfin/example/weather/internal/domain/entity"
	appRepo "surfin/example/weather/internal/repository"
	batch_config "surfin/pkg/batch/core/config"
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/core/adaptor"
	tx "surfin/pkg/batch/core/tx"
	"surfin/pkg/batch/support/util/exception"
	logger "surfin/pkg/batch/support/util/logger"
	
	configbinder "surfin/pkg/batch/support/util/configbinder"
)

// Writer固有の設定構造体 (JSLプロパティバインディング用)
type WeatherWriterConfig struct {
	TargetDBName string `yaml:"targetDBName,omitempty"`
	Database     string `yaml:"database,omitempty"` // 互換性のため
}

// WeatherItemWriter implements core.ItemWriter for writing weather data.
type WeatherItemWriter struct {
	TxManager tx.TransactionManager
	Repo      appRepo.WeatherRepository // Repository resolved at runtime.
	
	AllDBConnections   map[string]adaptor.DBConnection
	DBResolver         port.DBConnectionResolver
	Config             *batch_config.Config
	TargetDBName       string // Target DB name resolved from JSL properties. 
	
	// stepExecutionContext holds the reference to the Step's ExecutionContext.
	stepExecutionContext model.ExecutionContext 
	// writerState holds the writer's internal state.
	writerState          model.ExecutionContext
	resolver             port.ExpressionResolver
	dbResolver           port.DBConnectionResolver
}

// Verify that WeatherItemWriter implements the core.ItemWriter[any] interface.
var _ port.ItemWriter[any] = (*WeatherItemWriter)(nil)

// NewWeatherWriter creates a new WeatherItemWriter instance.
func NewWeatherWriter(
	cfg *batch_config.Config,
	allDBConnections map[string]adaptor.DBConnection,
	allTxManagers map[string]tx.TransactionManager,
	resolver port.ExpressionResolver,
	dbResolver port.DBConnectionResolver,
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
	
	// TxManager is mandatory.
	txManager, ok := allTxManagers[dbName]
	if !ok {
		return nil, fmt.Errorf("transaction manager '%s' not found", dbName)
	}
	
	// Check for DBConnection existence (used during Open).
	_, ok = allDBConnections[dbName]
	if !ok {
		return nil, fmt.Errorf("database connection '%s' not found", dbName)
	}


	return &WeatherItemWriter{
		TxManager: txManager,
		// Repo is initialized in Open.
		
		AllDBConnections: allDBConnections,
		DBResolver: dbResolver,
		Config: cfg,
		TargetDBName: dbName,
		
		resolver:  resolver,
		stepExecutionContext: model.NewExecutionContext(),
		writerState:          model.NewExecutionContext(), 
	}, nil
}

// Open opens resources and restores state from ExecutionContext.
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

// Write writes a chunk of items to the database within the provided transaction.
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
		return exception.NewBatchError("weather_writer", "failed to bulk insert weather data", err, true, true) // Changed isRetryable to true based on common DB error handling
	}

	logger.Debugf("Saved chunk of weather data items to the database. Count: %d", len(finalDataToStore))
	return nil
}

// Close releases resources.
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

// SetExecutionContext sets the ExecutionContext and restores the writer's state.
func (w *WeatherItemWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w.stepExecutionContext = ec
	return w.restoreWriterStateFromExecutionContext(ctx)
}

// GetExecutionContext retrieves the writer's ExecutionContext state.
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
	// 現在、writerCtx に保存するwriter固有の内部状態はありません。
	// 必要であればここに記述します。

	// Set the decision.condition for the job flow directly on the stepExecutionContext.
	// This key will be promoted to JobExecutionContext by the framework if configured in JSL.
	// IMPORTANT: Also set it on w.writerState so that GetExecutionContext returns it.
	writerCtx.Put("decision.condition", "true")
	w.writerState.Put("decision.condition", "true") // ADDED: w.writerState にも設定

	logger.Debugf("WeatherWriter: Internal state saved to ExecutionContext.")
	return nil
}
