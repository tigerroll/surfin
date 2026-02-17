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

	"github.com/tigerroll/surfin/pkg/batch/adapter/database" // Imports the generic database adapter interface.
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/dummy" // For temporary dummy DBConnectionResolver
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/mitchellh/mapstructure"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/mysql"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/postgres"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/sqlite" // GORM SQLite provider
	migrationfs "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration/filesystem"

	"go.uber.org/fx"
)

// init ensures that the coreAdapter package is explicitly used,
// preventing "imported and not used" compiler errors when its types are only
// referenced in comments or interfaces.
func init() {
	var _ coreAdapter.ResourceConnection // Explicitly reference a type from coreAdapter.
}

// DBProviderMap maps database type strings to functions that create database.DBProvider instances.
// This map is used by main.go to dynamically select and register database providers.
var DBProviderMap = map[string]func(cfg *config.Config) database.DBProvider{
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
//	WeatherAppFS: The embedded file system for application-specific migrations, tagged as "weatherAppFS".
//	FrameworkFS: The embedded file system for framework-specific migrations, tagged as "frameworkMigrationsFS".
type MigrationFSMapParams struct {
	fx.In
	// WeatherAppFS is the embedded file system for application-specific migrations, provided by an anonymous provider within this module.
	WeatherAppFS fs.FS `name:"weatherAppFS"`
	// FrameworkFS is the embedded file system for framework-specific migrations, provided by [migrationfs.Module].
	FrameworkFS fs.FS `name:"frameworkMigrationsFS"`
}

// NewMigrationFSMap aggregates all necessary migration file systems into a single map.
// This map is then used by the MigrationTasklet to locate migration scripts.
//
// Parameters:
//
//	p: MigrationFSMapParams containing injected file systems.
//
// Returns:
//
//	A map where keys are logical names for migration file systems and values are fs.FS instances.
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
	// DBProviders is a slice of all DBProvider implementations, automatically collected by Fx due to the `group:"db_providers"` tag.
	DBProviders []database.DBProvider `group:"db_providers"`
	// TxFactory is the TransactionManagerFactory used to create transaction managers.
	TxFactory tx.TransactionManagerFactory
}

// NewDBConnectionsAndTxManagers establishes connections for all data sources defined in the configuration file,
// using the appropriate DBProvider, and provides them as maps.
//
// Returns:
//   - A map of database connections (map[string]database.DBConnection), keyed by their configuration name.
//   - A map of database providers (map[string]database.DBProvider), keyed by their database type.
//   - An error if any connection establishment or configuration decoding fails.
func NewDBConnectionsAndTxManagers(p DBConnectionsAndTxManagersParams) (
	map[string]database.DBConnection,
	map[string]database.DBProvider,
	error,
) {
	allConnections := make(map[string]database.DBConnection)
	// Removed the declaration of allTxManagers.
	allProviders := make(map[string]database.DBProvider)

	// Map providers by DB type
	providerMap := make(map[string]database.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
		allProviders[provider.Type()] = provider // Store providers by DB type
	}

	// Extract database configurations from the main config
	// The AdapterConfigs map holds configurations keyed by adapter type (e.g., "database").
	// We need to get the "database" entry, which is itself a map of named database configurations.
	rawAdapterConfig, ok := p.Cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("invalid 'adapter' configuration format: expected map[string]interface{}")
	}

	dbAdapterConfig, ok := rawAdapterConfig["database"]
	if !ok {
		logger.Warnf("No 'database' adapter configuration found. Skipping database connection setup.")
		return allConnections, allProviders, nil
	}

	dbConfigsMap, ok := dbAdapterConfig.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("invalid 'database' adapter configuration format: expected map[string]interface{}")
	}

	// Loop through all named database configurations
	for name, rawConfig := range dbConfigsMap {
		var dbConfig dbconfig.DatabaseConfig
		if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
			return nil, nil, fmt.Errorf("failed to decode database config for '%s': %w", name, err)
		}

		var conn database.DBConnection
		// Handle dummy type explicitly: provide dummy implementations instead of skipping.
		if dbConfig.Type == "dummy" {
			logger.Infof("DB connection '%s' is configured as 'dummy'. Providing dummy implementations.", name)
			conn = dummy.NewDummyDBConnection(name)
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
			connAsResource, err := provider.GetConnection(name)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get connection for '%s' using provider '%s': %w", name, provider.Type(), err)
			}
			var typeAssertOk bool
			conn, typeAssertOk = connAsResource.(database.DBConnection)
			if !typeAssertOk {
				return nil, nil, fmt.Errorf("connection '%s' is not a DBConnection type", name)
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
			for _, provider := range p.DBProviders {
				wg.Add(1)
				go func(p database.DBProvider) {
					defer wg.Done()
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

	return allConnections, allProviders, nil
}

// NewMetadataTxManager extracts the "metadata" TransactionManager from the map.
// This function is responsible for providing the TransactionManager specifically for metadata operations.
//
// Parameters:
//
//	p: An Fx parameter struct containing:
//	  - AllDBConnections: A map of all established database connections.
//	  - TxFactory: The [tx.TransactionManagerFactory] to create the [tx.TransactionManager].
//
// Returns:
//   - The TransactionManager for the "metadata" connection.
//   - An error if the "metadata" connection is not found or if TxManager creation fails.
func NewMetadataTxManager(p struct { // Renamed parameter 'p' to 'params' for clarity.
	fx.In
	// AllDBConnections is the map of all established database connections.
	AllDBConnections map[string]database.DBConnection
	// TxFactory is injected to create the [tx.TransactionManager].
	TxFactory tx.TransactionManagerFactory
}) (tx.TransactionManager, error) {
	conn, ok := p.AllDBConnections["metadata"]
	if !ok { // Check if the "metadata" connection exists.
		return nil, fmt.Errorf("metadata DBConnection not found in aggregated map")
	}
	// Create TxManager using the injected TxFactory.
	return p.TxFactory.NewTransactionManager(conn), nil
}

// Module defines the application's Fx module. It configures and provides various components for the batch framework,
// including database connections, transaction managers, migration file systems, and the DB connection resolver.
var Module = fx.Options(
	// DB Provider Modules.
	// [gormadapter.Module] provides [gormadapter.NewGormTransactionManagerFactory].
	gormadapter.Module,
	// Add specific DB provider modules here
	mysql.Module,    // Adds the MySQL DBProvider to the Fx graph.
	postgres.Module, // Adds the PostgreSQL DBProvider to the Fx graph.
	sqlite.Module,   // SQLite DBProvider is added to the Fx graph.

	// Provide the aggregated map[string]database.DBConnection and map[string]tx.TransactionManager
	// NewDBConnectionsAndTxManagers also provides map[string]database.DBProvider
	fx.Provide(NewDBConnectionsAndTxManagers), // Provides map[string]database.DBConnection and map[string]database.DBProvider.
	// Provide the specific metadata TxManager required by JobFactory
	fx.Provide(fx.Annotate(
		NewMetadataTxManager,
		fx.ResultTags(`name:"metadata"`), // Tagged as "metadata" for injection into JobFactoryParams.
	)), // Note: An explicit fx.Provide for TxFactory is not needed here because gormadapter.Module already provides it. Fx automatically resolves TxFactory as a dependency for NewMetadataTxManager.

	fx.Provide( // Provide application migration FS by name.
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
				} // Return fs.FS.
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

	// Provide the default DBConnectionResolver (dummy implementation for now)
	fx.Provide(fx.Annotate(
		dummy.NewDefaultDBConnectionResolver,
	)),
)
