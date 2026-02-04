// main package provides the main application entry point and sets up the Fx dependency injection graph.
// It defines the application's configuration, dummy database implementations for a DB-less environment,
// and registers various batch components and listeners.
package main

import (
	// Standard library imports
	"context"
	"database/sql"
	"io/fs"
	"os"

	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	dummy "github.com/tigerroll/surfin/pkg/batch/adapter/database/dummy"
	// Batch framework imports
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	migration "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	decision "github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	split "github.com/tigerroll/surfin/pkg/batch/core/job/split"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	incrementer "github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	inmemoryRepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/inmemory"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"

	appjob "github.com/tigerroll/surfin/example/hello-world/internal/app/job"
	apprunner "github.com/tigerroll/surfin/example/hello-world/internal/app/runner"
)

// dummyMigrator is a dummy implementation of the migration.Migrator interface.
// It performs no actual migration operations, as the hello-world application
// does not require real database migrations.
type dummyMigrator struct{}

func (d *dummyMigrator) Up(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Up called, doing nothing.")
	return nil
}
func (d *dummyMigrator) Down(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Down called, doing nothing.")
	return nil
}
func (d *dummyMigrator) Close() error {
	logger.Debugf("Dummy Migrator: Close called, doing nothing.")
	return nil
}

// dummyMigratorProvider is a dummy implementation of the migration.MigratorProvider interface.
// It provides dummy Migrator instances, as real migrations are not needed
// for the hello-world application.
type dummyMigratorProvider struct{}

func (d *dummyMigratorProvider) NewMigrator(conn adapter.DBConnection) migration.Migrator {
	return &dummyMigrator{}
}

// dummyDBConnection is a dummy implementation of the adapter.DBConnection interface.
// It performs no actual database operations, as the hello-world application
// runs in a DB-less mode.
type dummyDBConnection struct{}

// ExecuteUpdate is a dummy implementation of the DBExecutor.ExecuteUpdate method.
func (d *dummyDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpdate called, doing nothing.")
	return 0, nil
}

// ExecuteUpsert is a dummy implementation of the DBExecutor.ExecuteUpsert method.
func (d *dummyDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, uniqueColumns []string, updateColumns []string) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpsert called, doing nothing. Table: %s", tableName)
	return 0, nil
}

// ExecuteQuery is a dummy implementation of the DBExecutor.ExecuteQuery method.
func (d *dummyDBConnection) ExecuteQuery(ctx context.Context, model interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: ExecuteQuery called, doing nothing. Query: %v", query)
	return nil
}

// Count is a dummy implementation of the DBExecutor.Count method.
func (d *dummyDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: Count called, doing nothing. Query: %v", query)
	return 0, nil
}

// ExecuteQueryAdvanced is a dummy implementation of the DBExecutor.ExecuteQueryAdvanced method.
func (d *dummyDBConnection) ExecuteQueryAdvanced(ctx context.Context, model interface{}, query map[string]interface{}, orderBy string, limit int) error {
	logger.Debugf("Dummy DBConnection: ExecuteQueryAdvanced called, doing nothing. Query: %v, OrderBy: %s, Limit: %d", query, orderBy, limit)
	return nil
}

// Pluck is a dummy implementation of the DBExecutor.Pluck method.
func (d *dummyDBConnection) Pluck(ctx context.Context, dest interface{}, field string, value interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: Pluck called, doing nothing. Field: %s, Value: %v, Query: %v", field, value, query)
	return nil
}

// RefreshConnection is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) RefreshConnection(ctx context.Context) error {
	logger.Debugf("Dummy DBConnection: RefreshConnection called, doing nothing.")
	return nil
}

// Type returns the type of the dummy database connection.
func (d *dummyDBConnection) Type() string { return "dummy" }

// Name returns the name of the dummy database connection.
func (d *dummyDBConnection) Name() string { return "dummy" }

// Close closes the dummy database connection.
func (d *dummyDBConnection) Close() error { return nil }

// IsTableNotExistError checks if the given error indicates that a table does not exist (always false for dummy).
func (d *dummyDBConnection) IsTableNotExistError(err error) bool { return false }

// Config is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) Config() dbconfig.DatabaseConfig { return dbconfig.DatabaseConfig{} }

// GetSQLDB is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) GetSQLDB() (*sql.DB, error) {
	return nil, nil // Returns nil as there's no actual SQL DB.
}

// dummyDBProvider is a dummy implementation of the adapter.DBProvider interface.
// It always returns dummy DBConnection instances.
type dummyDBProvider struct{}

// GetConnection returns a dummy DBConnection.
func (d *dummyDBProvider) GetConnection(name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: GetConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// ForceReconnect returns a new dummy DBConnection, simulating a re-establishment.
func (d *dummyDBProvider) ForceReconnect(name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: ForceReconnect called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// CloseAll performs no operation for dummy connections.
func (d *dummyDBProvider) CloseAll() error {
	logger.Debugf("Dummy DBProvider: CloseAll called.")
	return nil
}

// Type returns the type of the dummy database provider.
func (d *dummyDBProvider) Type() string { return "dummy" }

// dummyPortDBConnectionResolver is a dummy implementation of the port.DBConnectionResolver interface.
// It's used to satisfy dependencies in a DB-less environment.
type dummyPortDBConnectionResolver struct{}

// ResolveDBConnectionName returns the default connection name for dummy operations.
func (d *dummyPortDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnectionName called, returning default '%s'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnection returns a dummy DBConnection instance.
func (d *dummyPortDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// realEnvironmentExpander implements the config.EnvironmentExpander interface
// by expanding environment variables using os.ExpandEnv.
type realEnvironmentExpander struct{}

// Expand implements the Expand method of the config.EnvironmentExpander interface.
func (r *realEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	expanded := os.ExpandEnv(string(input))
	logger.Debugf("Real EnvironmentExpander: Expanded '%s' to '%s'", string(input), expanded)
	return []byte(expanded), nil
}

// provideRealEnvironmentExpander provides a real implementation of config.EnvironmentExpander.
func provideRealEnvironmentExpander() config.EnvironmentExpander {
	return &realEnvironmentExpander{}
}

// GetApplicationOptions constructs and returns a slice of uber-fx options.
// This function must be defined before the fx.New call.
func GetApplicationOptions(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes) []fx.Option {
	cfg, err := config.LoadConfig(envFilePath, embeddedConfig) // Load application configuration.
	if err != nil {                                            // Fatal error if configuration loading fails.
		logger.Fatalf("Failed to load configuration: %v", err) // Log and exit if config loading fails.
	}
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

	var options []fx.Option

	options = append(options, fx.Supply(
		embeddedConfig,
		embeddedJSL,
		fx.Annotate(envFilePath, fx.ResultTags(`name:"envFilePath"`)),
		cfg,
		fx.Annotate(appCtx, fx.As(new(context.Context)), fx.ResultTags(`name:"appCtx"`)),
	))
	// Dummy providers to satisfy framework migration dependencies.
	options = append(options, fx.Provide(func() migration.MigratorProvider { return &dummyMigratorProvider{} }))
	options = append(options, fx.Provide(fx.Annotate(
		func() map[string]fs.FS { return make(map[string]fs.FS) },
		fx.ResultTags(`name:"allMigrationFS"`),
	)))
	// Add dummy provider for DBConnectionResolver. dummyPortDBConnectionResolver implements adapter.DBConnectionResolver.
	options = append(options, fx.Provide(func() adapter.DBConnectionResolver { return &dummyPortDBConnectionResolver{} }))
	options = append(options, fx.Provide(func() map[string]adapter.DBProvider {
		return map[string]adapter.DBProvider{
			"default":  &dummyDBProvider{}, // Provides at least one dummy provider.
			"metadata": &dummyDBProvider{}, // Adds dummy provider for "metadata" database for framework migrations.
			"dummy":    &dummyDBProvider{}, // Adds dummy provider for "dummy" database for JobRepositoryDBRef.
		}
	}))
	// Add dummy providers for missing transaction manager-related dependencies.
	options = append(options, fx.Provide(func() tx.TransactionManagerFactory {
		return &dummy.DummyTxManagerFactory{}
	}))
	options = append(options, fx.Provide(func() map[string]tx.TransactionManager {
		return make(map[string]tx.TransactionManager) // Provides an empty map to satisfy NewMetadataTxManager's dependencies.
	}))
	options = append(options, fx.Provide(fx.Annotate(dummy.NewMetadataTxManager, fx.ResultTags(`name:"metadata"`))))

	// Add real provider for config.EnvironmentExpander
	options = append(options, fx.Provide(provideRealEnvironmentExpander))

	options = append(options, logger.Module)
	options = append(options, metrics.Module)
	options = append(options, bootstrap.Module)
	options = append(options, fx.Provide(supportConfig.NewJobFactory))
	options = append(options, usecase.Module)
	options = append(options, inmemoryRepo.Module)
	options = append(options, batchlistener.Module)
	options = append(options, decision.Module)
	options = append(options, split.Module)
	options = append(options, apprunner.Module)
	options = append(options, incrementer.Module)
	options = append(options, item.Module)
	options = append(options, fx.Invoke(fx.Annotate(startJobExecution, fx.ParamTags("", "", "", "", "", `name:"appCtx"`))))
	options = append(options, helloTasklet.Module) // Include the module for HelloWorldTasklet.
	options = append(options, appjob.Module)       // Directly include the module that provides application-specific JobBuilders.

	return options
}
