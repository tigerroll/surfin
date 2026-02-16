package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database" // Imports the database package.
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/support/expression"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/factory"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Module provides bootstrap-related components to Fx.
var Module = fx.Options(
	fx.Provide(NewBatchInitializer),   // Provides the BatchInitializer.
	fx.Invoke(LoadJSLDefinitionsHook), // Registers a lifecycle hook to load JSL definitions.
	fx.Invoke(ApplyLoggingConfigHook), // Registers a lifecycle hook to apply logging configuration.

	expression.Module, // Provides the ExpressionResolver.

	// Engine Components: Provides Step Factory, StepExecutor, Retry Module, Skip Module.
	factory.Module,
	partition.Module,

	// Registers a lifecycle hook to run framework migrations at application startup.
	fx.Invoke(runFrameworkMigrationsHook),
)

// RunFrameworkMigrationsHookParams defines the dependencies required for the
// runFrameworkMigrationsHook function, injected by Fx.
type RunFrameworkMigrationsHookParams struct {
	fx.In
	Lifecycle        fx.Lifecycle                   // The Fx lifecycle to append hooks.
	Cfg              *config.Config                 // The application configuration.
	MigratorProvider migration.MigratorProvider     // Provider for database migrators.
	AllMigrationFS   map[string]fs.FS               `name:"allMigrationFS"` // A map of file systems containing migration scripts, including "frameworkMigrationsFS".
	AllDBProviders   map[string]database.DBProvider // All registered DB providers, mapped by their database type.
}

// runFrameworkMigrationsHook registers an Fx lifecycle hook to execute necessary
// framework migrations at application startup.
func runFrameworkMigrationsHook(
	p RunFrameworkMigrationsHookParams,
) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// If JobRepositoryDBRef is not configured, framework migrations will be skipped.
			if p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef == "" {
				logger.Warnf("JobRepositoryDBRef is not configured. Framework migrations will be skipped.")
				return nil
			}

			logger.Infof("Running framework migrations for JobRepository database: %s", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)

			// Get DB Configuration to determine DB Type
			var dbConfig dbconfig.DatabaseConfig
			rawAdapterConfig, ok := p.Cfg.Surfin.AdapterConfigs.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid 'adapter' configuration format: expected map[string]interface{}")
			}
			adapterConfig, ok := rawAdapterConfig["database"]
			if !ok {
				return fmt.Errorf("no 'database' adapter configuration found in Surfin.AdapterConfigs")
			}
			dbConfigsMap, ok := adapterConfig.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid 'database' adapter configuration format: expected map[string]interface{}")
			}
			rawConfig, ok := dbConfigsMap[p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef]
			if !ok {
				return fmt.Errorf("database configuration '%s' not found under 'adapter.database' configs for JobRepositoryDBRef", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
			}
			if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
				return fmt.Errorf("failed to decode database config for '%s': %w", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef, err)
			}

			// If JobRepositoryDBRef is configured as 'dummy' type, skip framework migrations.
			// This supports scenarios like DB-less mode or testing where an actual database
			// connection is not expected.
			if strings.TrimSpace(strings.ToLower(dbConfig.Type)) == "dummy" {
				logger.Infof("JobRepositoryDBRef '%s' is configured as 'dummy'. Skipping framework migrations.", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
				return nil
			}

			dbProvider, ok := p.AllDBProviders[dbConfig.Type]
			if !ok {
				return fmt.Errorf("no DBProvider found for database type '%s'", dbConfig.Type)
			}
			dbConn, err := dbProvider.GetConnection(p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
			if err != nil {
				return fmt.Errorf("failed to get DB connection for framework migrations: %w", err)
			}

			frameworkFS, ok := p.AllMigrationFS["frameworkMigrationsFS"]
			if !ok {
				return fmt.Errorf("frameworkMigrationsFS not found in allMigrationFS map")
			}

			migrator := p.MigratorProvider.NewMigrator(dbConn)
			// Use DB type as the migration directory name (e.g., "mysql", "postgres", "sqlite").
			migrationDir := dbConn.Type()

			migrationErr := migrator.Up(ctx, frameworkFS, migrationDir, "batch_framework_migrations")

			// Regardless of migration success or failure, force re-establishment of the JobRepositoryDBRef's DB connection.
			// This ensures that the connection used by JobRepository is always fresh and valid.
			// Re-decode the config to ensure we have the latest, correct structure.

			if migrationErr != nil {
				// Handle "no change" error specially as it's not an actual error.
				if migrationErr.Error() == "no change" {
					logger.Infof("Framework migrations for %s: No new migrations to apply.", dbConn.Name())
					return nil
				}
				// Return the migration error itself.
				return fmt.Errorf("failed to execute framework migrations for %s: %w", dbConn.Name(), migrationErr)
			}
			logger.Infof("Framework migrations for %s completed successfully.", dbConn.Name())
			return nil
		},
	})
}
