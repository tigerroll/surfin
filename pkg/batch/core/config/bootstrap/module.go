package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/fx"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
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
	Lifecycle        fx.Lifecycle                  // The Fx lifecycle to append hooks.
	Cfg              *config.Config                // The application configuration.
	MigratorProvider migration.MigratorProvider    // Provider for database migrators.
	DBResolver       port.DBConnectionResolver     // Resolver for database connections.
	AllMigrationFS   map[string]fs.FS              `name:"allMigrationFS"` // A map of file systems containing migration scripts, including "frameworkMigrationsFS".
	AllDBProviders   map[string]adapter.DBProvider // All registered DB providers, mapped by their database type.
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
			rawConfig, ok := p.Cfg.Surfin.AdapterConfigs[p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef]
			if !ok {
				return fmt.Errorf("database configuration '%s' not found in adapter.database configs for JobRepositoryDBRef", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
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

			dbConn, err := p.DBResolver.ResolveDBConnection(ctx, p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
			if err != nil {
				return fmt.Errorf("failed to resolve DB connection for framework migrations: %w", err)
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
			rawConfig, ok = p.Cfg.Surfin.AdapterConfigs[p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef]
			if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
				return fmt.Errorf("failed to decode database config for '%s' during reconnect: %w", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef, err)
			}
			provider, ok := p.AllDBProviders[dbConfig.Type]
			if !ok {
				// Handle special cases like Redshift
				if dbConfig.Type == "redshift" {
					provider, ok = p.AllDBProviders["postgres"]
				}
				if !ok {
					return fmt.Errorf("DBProvider for type '%s' not found for JobRepositoryDBRef", dbConfig.Type)
				}
			}

			// Before calling ForceReconnect, verify that the target connection actually exists.
			// Connections should already be established by NewDBConnectionsAndTxManagers, but this is a safeguard.
			_, connExists := p.AllDBProviders[dbConfig.Type]
			if !connExists {
				// If the connection does not exist, treat it as an error.
				return fmt.Errorf("DBProvider for type '%s' does not manage connection '%s'", dbConfig.Type, p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
			}

			// This closes any old connections (if open) and establishes a new one.
			// The returned DBConnection is ignored, as the DBConnectionResolver is expected to
			// retrieve the refreshed connection from the provider's internal state.
			_, forceReconnectErr := provider.ForceReconnect(p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)
			// Note: The returned DBConnection is not directly used here, as the DBConnectionResolver
			// is expected to retrieve the refreshed connection from the provider's internal state
			// when JobRepository requests it.
			if forceReconnectErr != nil {
				logger.Errorf("Failed to reconnect DB connection '%s' after migration: %v", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef, forceReconnectErr)
				// If reconnection fails, return it as a fatal error.
				return fmt.Errorf("failed to reconnect DB connection after migration: %w", forceReconnectErr)
			}
			logger.Infof("Forced reconnection of DB '%s' after migration.", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)

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
