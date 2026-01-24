package bootstrap

import (
	"context"
	"fmt"
	"io/fs"

	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/support/expression"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/factory"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// Module provides initializer-related components to Fx.
var Module = fx.Options(
	fx.Provide(NewBatchInitializer),   // Defined in initializer.go
	fx.Invoke(LoadJSLDefinitionsHook), // Defined in initializer.go
	fx.Invoke(ApplyLoggingConfigHook), // Defined in initializer.go (Corresponds to A_LOG_1)

	expression.Module, // Provides Expression Resolver

	// Engine Components (Provides Step Factory, StepExecutor, Retry Module, Skip Module)
	factory.Module,
	partition.Module,

	// Add a hook to run framework migrations at application startup.
	fx.Invoke(runFrameworkMigrationsHook), 
)

// RunFrameworkMigrationsHookParams defines the dependencies for runFrameworkMigrationsHook.
type RunFrameworkMigrationsHookParams struct {
	fx.In
	Lifecycle        fx.Lifecycle
	Cfg              *config.Config
	MigratorProvider migration.MigratorProvider
	DBResolver       adaptor.DBConnectionResolver
	AllMigrationFS   map[string]fs.FS `name:"allMigrationFS"` // This map includes "frameworkMigrationsFS".
	AllDBProviders   map[string]adaptor.DBProvider // All registered DB providers, mapped by type.
}

// runFrameworkMigrationsHook executes necessary framework migrations at application startup.
func runFrameworkMigrationsHook(
	p RunFrameworkMigrationsHookParams,
) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// JobRepositoryDBRef is not configured. Framework migrations will be skipped.
			if p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef == "" {
				logger.Warnf("JobRepositoryDBRef is not configured. Framework migrations will be skipped.")
				return nil
			}

			logger.Infof("Running framework migrations for JobRepository database: %s", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef)

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
			dbConfig, ok := p.Cfg.Surfin.Datasources[p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef]
			if !ok {
				return fmt.Errorf("database configuration '%s' not found for JobRepositoryDBRef", p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef) 
			}
			provider, ok := p.AllDBProviders[dbConfig.Type]
			if !ok {
				// Handle special cases like Redshift (similar to DefaultDBConnectionResolver).
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
