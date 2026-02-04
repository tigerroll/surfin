// Package main provides the main application entry point and sets up the Fx dependency injection graph.
// It defines the application's configuration, dummy database implementations for a DB-less environment,
// and registers various batch components and listeners.
package main

import (
	// Standard library imports
	"context"
	"io/fs"
	"os"

	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	database "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dummy "github.com/tigerroll/surfin/pkg/batch/adapter/database/dummy"
	// Batch framework imports
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	migration "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
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
	logger.Debugf("Dummy Migrator: Down called, doing nothing.") // Logs that the dummy Down method was called.
	return nil
}
func (d *dummyMigrator) Close() error {
	logger.Debugf("Dummy Migrator: Close called, doing nothing.") // Logs that the dummy Close method was called.
	return nil
}

// dummyMigratorProvider is a dummy implementation of the migration.MigratorProvider interface.
// It provides dummy Migrator instances, as real migrations are not needed
// for the hello-world application.
type dummyMigratorProvider struct{}

// NewMigrator returns a new dummy Migrator instance.
//
// Parameters:
//   - conn: The database connection (ignored by this dummy implementation).
//
// Returns:
//   - A dummy Migrator instance.
func (d *dummyMigratorProvider) NewMigrator(conn database.DBConnection) migration.Migrator {
	return &dummyMigrator{}
}

// dummyPortDBConnectionResolver is a dummy implementation of the database.DBConnectionResolver interface.
// It is used to satisfy dependencies in a DB-less environment.
type dummyPortDBConnectionResolver struct{}

// ResolveConnection is a method embedded from coreAdapter.ResourceConnectionResolver.
// It returns a dummy ResourceConnection.
//
// Parameters:
//   - ctx: The context for the operation.
//   - name: The name of the connection to resolve.
//
// Returns:
//   - A dummy ResourceConnection and nil error.
func (d *dummyPortDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveConnection called for '%s'.", name)
	// Since database.DBConnection embeds coreAdapter.ResourceConnection,
	// dummy.NewDummyDBConnection() also functions as a ResourceConnection.
	return dummy.NewDummyDBConnection(), nil
}

// ResolveConnectionName is a method embedded from coreAdapter.ResourceConnectionResolver.
// It returns the default connection name.
//
// Parameters:
//   - ctx: The context for the operation.
//   - jobExecution: The current JobExecution (ignored by this dummy implementation).
//   - stepExecution: The current StepExecution (ignored by this dummy implementation).
//   - defaultName: The default name to return.
//
// Returns:
//   - The default connection name and nil error.
func (d *dummyPortDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveConnectionName called, returning default '%v'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnectionName returns the default connection name.
//
// Parameters:
//   - ctx: The context for the operation.
//   - jobExecution: The current JobExecution (ignored by this dummy implementation).
//   - stepExecution: The current StepExecution (ignored by this dummy implementation).
//   - defaultName: The default name to return.
//
// Returns:
//   - The default connection name and nil error.
func (d *dummyPortDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnectionName called, returning default '%s'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnection returns a dummy DBConnection instance.
//
// Parameters:
//   - ctx: The context for the operation.
//   - name: The name of the connection to resolve (ignored by this dummy implementation).
//
// Returns:
//   - A dummy DBConnection instance and nil error.
func (d *dummyPortDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (database.DBConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnection called for '%s'.", name)
	return dummy.NewDummyDBConnection(), nil
}

// realEnvironmentExpander implements the config.EnvironmentExpander interface
// by expanding environment variables using os.ExpandEnv.
type realEnvironmentExpander struct{}

// Expand implements the Expand method of the config.EnvironmentExpander interface.
func (r *realEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	expanded := os.ExpandEnv(string(input))
	logger.Debugf("Real EnvironmentExpander: Expanded '%s' to '%s'", string(input), expanded) // Logs the expansion process.
	return []byte(expanded), nil
}

// provideRealEnvironmentExpander provides a real implementation of config.EnvironmentExpander.
//
// Returns:
//   - A real implementation of the config.EnvironmentExpander interface.
func provideRealEnvironmentExpander() config.EnvironmentExpander {
	return &realEnvironmentExpander{}
}

// GetApplicationOptions constructs and returns a slice of uber-fx options.
// This function must be defined before the fx.New call.
//
// Parameters:
//   - appCtx: The application context.
//   - envFilePath: The path to the environment configuration file.
//   - embeddedConfig: Embedded configuration bytes.
//   - embeddedJSL: Embedded JSL definition bytes.
//
// Returns:
//   - A slice of fx.Option to configure the Fx application.
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
	// Provides a dummy implementation of migration.MigratorProvider.
	options = append(options, fx.Provide(func() migration.MigratorProvider { return &dummyMigratorProvider{} }))
	// Provides an empty map for all migration file systems, as no real migrations are performed.
	options = append(options, fx.Provide(fx.Annotate(
		func() map[string]fs.FS { return make(map[string]fs.FS) },
		fx.ResultTags(`name:"allMigrationFS"`),
	)))
	// Adds a dummy provider for DBConnectionResolver.
	// dummyPortDBConnectionResolver implements both database.DBConnectionResolver and coreAdapter.ResourceConnectionResolver.
	options = append(options, fx.Provide(fx.Annotate(
		func() *dummyPortDBConnectionResolver { return &dummyPortDBConnectionResolver{} },
		fx.As(new(database.DBConnectionResolver)),
		fx.As(new(coreAdapter.ResourceConnectionResolver)),
	)))
	// Provides a map of dummy DBProvider instances for various connection names.
	options = append(options, fx.Provide(func() map[string]database.DBProvider {
		return map[string]database.DBProvider{
			"default":  dummy.NewDummyDBProvider(),
			"metadata": dummy.NewDummyDBProvider(),
			"dummy":    dummy.NewDummyDBProvider(),
		}
	}))
	// Add dummy providers for missing transaction manager-related dependencies.
	// Provides a dummy TransactionManagerFactory.
	options = append(options, fx.Provide(func() tx.TransactionManagerFactory {
		return &dummy.DummyTxManagerFactory{}
	}))
	// Provides an empty map for TransactionManagers, as they are not used in this dummy setup.
	options = append(options, fx.Provide(func() map[string]tx.TransactionManager {
		return make(map[string]tx.TransactionManager) // Provides an empty map to satisfy NewMetadataTxManager's dependencies.
	}))
	// Provides a dummy MetadataTxManager.
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
