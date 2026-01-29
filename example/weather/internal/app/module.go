// Package app provides the main application module for the weather batch example.
// It sets up dependency injection for database connections, migration file systems,
// and other core components required for the application to run.
package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sync"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config"
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/mitchellh/mapstructure"
	dummy "github.com/tigerroll/surfin/pkg/batch/adaptor/database/dummy" // Dummy database implementations
	gormadaptor "github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/mysql"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/postgres"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/sqlite" // GORM SQLite provider
	migrationfs "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration/filesystem"

	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port" // Core application interfaces
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"    // Core domain models
)

// DBProviderMap is used by main.go to dynamically select providers.
var DBProviderMap = map[string]func(cfg *config.Config) adaptor.DBProvider{
	"postgres": postgres.NewProvider,
	"redshift": postgres.NewProvider, // Redshift also uses PostgresProvider
	"mysql":    mysql.NewProvider,
	"sqlite":   sqlite.NewProvider,
}

// MigrationFSMapParams defines the dependencies for NewMigrationFSMap.
//
// Parameters:
//
//	fx.In: Fx-injected parameters.
//	WeatherAppFS: The embedded file system for application-specific migrations.
//	FrameworkFS: The embedded file system for framework-specific migrations.
type MigrationFSMapParams struct {
	fx.In
	// WeatherAppFS is provided by the anonymous provider below.
	WeatherAppFS fs.FS `name:"weatherAppFS"`
	// FrameworkFS is provided by migrationfs.Module.
	FrameworkFS fs.FS `name:"frameworkMigrationsFS"`
}

// NewMigrationFSMap aggregates all necessary migration file systems into a single map.
func NewMigrationFSMap(p MigrationFSMapParams) map[string]fs.FS {
	fsMap := make(map[string]fs.FS)

	// 1. Add Framework FS
	// Framework FS is provided with the name "frameworkMigrationsFS"
	frameworkFSKey := "frameworkMigrationsFS"
	if p.FrameworkFS != nil { // Ensure the FS is not nil before adding.
		fsMap[frameworkFSKey] = p.FrameworkFS
	}

	// 2. Add Application FS
	if p.WeatherAppFS != nil {
		fsMap["weatherAppFS"] = p.WeatherAppFS
	}

	logger.Debugf("Aggregated %d total migration FSs into a map.", len(fsMap))
	return fsMap
}

// DBConnectionsAndTxManagersParams defines the dependencies for NewDBConnectionsAndTxManagers.
type DBConnectionsAndTxManagersParams struct {
	fx.In                    // Fx-injected parameters.
	Lifecycle fx.Lifecycle   // The Fx lifecycle for hook registration.
	Cfg       *config.Config // The application configuration.
	// DBProviders is a slice of all DBProvider implementations, automatically collected by Fx
	// due to the `group:"db_providers"` tag.
	DBProviders []adaptor.DBProvider `group:"db_providers"`
	// TxFactory is the TransactionManagerFactory used to create transaction managers.
	TxFactory tx.TransactionManagerFactory
}

// NewDBConnectionsAndTxManagers establishes connections and transaction managers for all data sources defined in the configuration file,
// using the appropriate DBProvider, and provides them as maps.
//
// Returns:
//   - A map of database connections, keyed by their configuration name.
//   - A map of database providers, keyed by their database type.
//   - An error if any connection establishment or configuration decoding fails.
func NewDBConnectionsAndTxManagers(p DBConnectionsAndTxManagersParams) (
	map[string]adaptor.DBConnection,
	map[string]adaptor.DBProvider, // Removed map[string]tx.TransactionManager as it's no longer directly provided here.
	error,
) {
	allConnections := make(map[string]adaptor.DBConnection)
	// Removed the declaration of allTxManagers.
	allProviders := make(map[string]adaptor.DBProvider)

	// Map providers by DB type
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
		allProviders[provider.Type()] = provider // Store providers by DB type
	}

	// Loop through all data sources defined in the configuration file
	for name, rawConfig := range p.Cfg.Surfin.AdaptorConfigs {
		var dbConfig dbconfig.DatabaseConfig
		if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
			return nil, nil, fmt.Errorf("failed to decode database config for '%s': %w", name, err)
		}

		var conn adaptor.DBConnection
		// Handle dummy type explicitly: provide dummy implementations instead of skipping.
		if dbConfig.Type == "dummy" {
			logger.Infof("DB connection '%s' is configured as 'dummy'. Providing dummy implementations.", name)
			conn = &dummy.DummyDBConnection{}
			// TxManager is now created on demand via TxManagerFactory.
		} else {
			provider, ok := providerMap[dbConfig.Type]
			if !ok {
				// PostgresProvider also handles Redshift, so strict checking is avoided here.
				if dbConfig.Type == "redshift" {
					provider, ok = providerMap["postgres"]
				}
				if !ok {
					logger.Warnf("No DBProvider found for database type '%s' (Datasource: %s). Skipping connection.", dbConfig.Type, name)
					continue // Still skip if no provider is found for non-dummy types
				}
			}

			// Get connection using the provider
			var err error
			conn, err = provider.GetConnection(name)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get connection for '%s' using provider '%s': %w", name, provider.Type(), err)
			}
		}

		allConnections[name] = conn
		logger.Debugf("Initialized DB Connection for: %s (%s)", name, dbConfig.Type)
	}

	// Add a hook to the Fx lifecycle to close all connections during shutdown.
	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Infof("Closing all database connections...")
			var wg sync.WaitGroup
			var lastErr error

			// Close connections for each provider
			for _, provider := range p.DBProviders {
				wg.Add(1)
				go func(p adaptor.DBProvider) {
					defer wg.Done()
					// Connection closing is delegated to the provider
					if err := p.CloseAll(); err != nil {
						logger.Errorf("Failed to close connections for provider %s: %v", p.Type(), err)
						lastErr = err
					}
				}(provider)
			}
			wg.Wait()
			return lastErr
		},
	})

	return allConnections, allProviders, nil // Removed allTxManagers from return.
}

// NewMetadataTxManager extracts the "metadata" TransactionManager from the map.
// This function is responsible for providing the TransactionManager specifically for metadata operations.
//
// Parameters:
//
//	p: An Fx parameter struct containing:
//	  - AllDBConnections: A map of all established database connections.
//	  - TxFactory: The TransactionManagerFactory to create the TransactionManager.
//
// Returns:
//   - The TransactionManager for the "metadata" connection.
//   - An error if the "metadata" connection is not found or if TxManager creation fails.
func NewMetadataTxManager(p struct {
	fx.In
	// AllDBConnections is the map of all established database connections.
	AllDBConnections map[string]adaptor.DBConnection
	// TxFactory is injected to create the TransactionManager.
	TxFactory tx.TransactionManagerFactory
}) (tx.TransactionManager, error) {
	conn, ok := p.AllDBConnections["metadata"] // Changed from AllTxManagers to AllDBConnections.
	if !ok {
		return nil, fmt.Errorf("metadata DBConnection not found in aggregated map") // Corrected error message.
	}
	// Create TxManager using the injected TxFactory.
	return p.TxFactory.NewTransactionManager(conn), nil
}

// DefaultDBConnectionResolver is the default implementation of port.DBConnectionResolver.
// It resolves database connection names and provides actual database connection instances.
type DefaultDBConnectionResolver struct {
	providers map[string]adaptor.DBProvider
	cfg       *config.Config
	// resolver is an ExpressionResolver used for dynamic name resolution.
	resolver port.ExpressionResolver
}

// NewDefaultDBConnectionResolver creates a new DefaultDBConnectionResolver.
// To resolve circular dependencies, DBProviders are received as a list and converted to a map internally.
//
// Parameters:
//
//	p: An Fx parameter struct containing:
//	  - DBProviders: A slice of all DBProvider implementations.
//	  - Cfg: The application configuration.
//	  - ExpressionResolver: The ExpressionResolver for dynamic name resolution.
//
// Returns:
//
//	A new instance of port.DBConnectionResolver.
func NewDefaultDBConnectionResolver(p struct {
	fx.In
	// DBProviders are received as a list and converted to a map internally.
	DBProviders []adaptor.DBProvider `group:"db_providers"`
	Cfg         *config.Config       // The application configuration.
	// ExpressionResolver is used for dynamic name resolution.
	ExpressionResolver port.ExpressionResolver
}) port.DBConnectionResolver {
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
	}
	return &DefaultDBConnectionResolver{
		providers: providerMap,
		cfg:       p.Cfg,
		resolver:  p.ExpressionResolver, // Set the resolver field.
	}
}

// ResolveDBConnectionName resolves the database connection name (e.g., "metadata", "workload") based on the execution context.
// This implementation can be extended to support dynamic connection name resolution defined in JSL.
// Currently, it checks for "dbConnectionName" in StepExecution's ExecutionContext.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution (can be nil).
//	stepExecution: The current StepExecution (can be nil).
//	defaultName: The default connection name to use if no dynamic resolution occurs.
//
// Returns:
//
//	The resolved database connection name.
//	An error if resolution fails.
func (r *DefaultDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	// This implementation is used to support dynamic connection name resolution logic defined in JSL.
	// Currently, it can simply return the default name or resolve from ExecutionContext.
	// Example: Use "dbConnectionName" if set in StepExecution's ExecutionContext.
	if stepExecution != nil {
		if connName, ok := stepExecution.ExecutionContext.GetString("dbConnectionName"); ok {
			return connName, nil
		}
	}
	// Logic to resolve from JobParameters can also be added.
	// if jobExecution != nil {
	// 	if connName, ok := jobExecution.JobParameters.Params["dbConnectionName"].(string); ok {
	// 		return connName, nil
	// 	}
	// }

	// Dynamic name resolution using ExpressionResolver is also possible.
	// For example, if defined in JSL as #{jobParameters['dbName']}.
	// if r.resolver != nil {
	// 	resolvedName, err := r.resolver.Resolve(ctx, defaultName, jobExecution, stepExecution)
	// 	if err == nil && resolvedName != "" {
	// 		return resolvedName, nil
	// 	}
	// }

	return defaultName, nil
}

// ResolveDBConnection resolves a database connection instance by name.
// This method is responsible for ensuring that the returned connection is valid and re-establishes
// the connection if necessary.
//
// Parameters:
//
//	ctx: The context for the operation.
//	name: The name of the database connection to resolve (e.g., "metadata", "workload").
//
// Returns:
//   - adaptor.DBConnection: The resolved database connection instance.
//   - error: An error that occurred during connection resolution or re-establishment.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error) {
	rawConfig, ok := r.cfg.Surfin.AdaptorConfigs[name]
	if !ok {
		return nil, fmt.Errorf("database configuration '%s' not found in adaptor.database configs", name)
	}
	var dbConfig dbconfig.DatabaseConfig
	if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil { // Decode the raw configuration into a DatabaseConfig struct.
		return nil, fmt.Errorf("failed to decode database config for '%s': %w", name, err)
	}

	// If the database type is "dummy", return a dummy connection.
	if dbConfig.Type == "dummy" {
		logger.Debugf("Returning dummy DB connection for '%s'.", name) // Log that a dummy connection is being returned.
		return &dummy.DummyDBConnection{}, nil
	}

	provider, ok := r.providers[dbConfig.Type]
	if !ok {
		// Consider special cases like Redshift
		if dbConfig.Type == "redshift" {
			provider, ok = r.providers["postgres"]
		}
		if !ok {
			return nil, fmt.Errorf("DBProvider for type '%s' not found", dbConfig.Type)
		}
	}

	// Get connection using the provider.
	// The provider manages the connection pool internally and ensures the connection is valid.
	conn, err := provider.GetConnection(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection '%s': %w", name, err)
	}

	// GetConnection is expected to return the latest valid connection.
	// conn.RefreshConnection(ctx) // Optional: Call RefreshConnection to explicitly ensure the connection is valid.

	return conn, nil
}

// Module defines the application's Fx module.
// It configures and provides various components for the batch framework,
// including database connections, transaction managers, migration file systems,
// and the DB connection resolver.
var Module = fx.Options(
	// DB Provider Modules
	// gormadaptor.Module provides NewGormTransactionManagerFactory.
	gormadaptor.Module,

	// Provide the aggregated map[string]adaptor.DBConnection and map[string]tx.TransactionManager
	// NewDBConnectionsAndTxManagers also provides map[string]adaptor.DBProvider
	fx.Provide(NewDBConnectionsAndTxManagers),
	// Provide the specific metadata TxManager required by JobFactory
	fx.Provide(fx.Annotate(
		NewMetadataTxManager,
		fx.ResultTags(`name:"metadata"`), // Tagged as "metadata" for injection into JobFactoryParams.
	)),
	// Note: An explicit fx.Provide for TxFactory is not needed here because gormadaptor.Module already provides it.
	// Fx automatically resolves TxFactory as a dependency for NewMetadataTxManager.

	// Provide the concrete DBConnectionResolver implementation
	fx.Provide(fx.Annotate(
		NewDefaultDBConnectionResolver,
		fx.As(new(port.DBConnectionResolver)), // Register as an implementation of port.DBConnectionResolver.
	)),

	// Provide application migration FS by name
	fx.Provide(
		fx.Annotate(
			func(params struct {
				fx.In // Fx-injected parameters.
				// RawAppMigrationsFS is the raw embedded file system injected from main.go.
				RawAppMigrationsFS embed.FS `name:"rawApplicationMigrationsFS"`
			}) fs.FS {
				// Due to 'go:embed all:resources/migrations', the 'resources' directory is created at the root of the FS.
				// Remove this prefix so the framework can directly reference 'postgres' or 'mysql'.
				subFS, err := fs.Sub(params.RawAppMigrationsFS, "resources/migrations")
				if err != nil {
					// The go:embed path is fixed, so this should not normally be reached, but panic just in case.
					logger.Fatalf("Failed to create subdirectory for application migration FS: %v", err) // Log fatal error if sub-directory creation fails.
				}
				// Return fs.FS.
				return subFS
			},
			// Tag the result with the name 'weatherAppFS'.
			fx.ResultTags(`name:"weatherAppFS"`), // Tag the result for specific injection.
		),
	),

	// Provide the aggregated map[string]fs.FS
	fx.Provide(fx.Annotate(
		NewMigrationFSMap,
		fx.ResultTags(`name:"allMigrationFS"`), // Tag the result for specific injection.
	)),
	// migrationfs.Module explicitly provides the framework migration FS on the application side.
	migrationfs.Module,
)
