package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sync"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	gormadaptor "github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/mysql"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/postgres"
	"github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm/sqlite"
	migrationfs "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration/filesystem"
	
	"go.uber.org/fx"
)

// DBProviderMap is used by main.go to dynamically select providers.
var DBProviderMap = map[string]func(cfg *config.Config) adaptor.DBProvider {
	"postgres": postgres.NewProvider,
	"redshift": postgres.NewProvider, // Redshift also uses PostgresProvider
	"mysql":    mysql.NewProvider,
	"sqlite":   sqlite.NewProvider,
}

// MigrationFSMapParams defines the dependencies for NewMigrationFSMap.
type MigrationFSMapParams struct {
	fx.In
	WeatherAppFS fs.FS `name:"weatherAppFS"` // Provided by the anonymous provider below
	FrameworkFS  fs.FS `name:"frameworkMigrationsFS"` // Provided by migrationfs.Module
}

// NewMigrationFSMap aggregates all necessary migration file systems into a single map.
func NewMigrationFSMap(p MigrationFSMapParams) map[string]fs.FS {
	fsMap := make(map[string]fs.FS)

	// 1. Add Framework FS
	// Framework FS is provided with the name "frameworkMigrationsFS"
	frameworkFSName := "frameworkMigrationsFS" 
	if p.FrameworkFS != nil {
		fsMap[frameworkFSName] = p.FrameworkFS 
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
	fx.In
	Lifecycle fx.Lifecycle
	Cfg       *config.Config
	// Fx automatically collects all components tagged with adaptor.DBProviderGroup into a slice.
	DBProviders []adaptor.DBProvider `group:"db_providers"`
	// Inject the TransactionManagerFactory
	TxFactory tx.TransactionManagerFactory
}

// NewDBConnectionsAndTxManagers establishes connections and transaction managers for all data sources defined in the configuration file,
// using the appropriate DBProvider, and provides them as maps.
func NewDBConnectionsAndTxManagers(p DBConnectionsAndTxManagersParams) (
	map[string]adaptor.DBConnection,
	map[string]tx.TransactionManager,
	map[string]adaptor.DBProvider, // Also provide a map of DBProviders
	error,
) {
	allConnections := make(map[string]adaptor.DBConnection)
	allTxManagers := make(map[string]tx.TransactionManager)
	allProviders := make(map[string]adaptor.DBProvider)
	
	// Map providers by DB type
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
		allProviders[provider.Type()] = provider // Store providers by DB type
	}

	// Loop through all data sources defined in the configuration file
	for name, dbConfig := range p.Cfg.Surfin.Datasources {
		provider, ok := providerMap[dbConfig.Type]
		if !ok {
			// PostgresProvider also handles Redshift, so strict checking is avoided here.
			if dbConfig.Type == "redshift" {
				provider, ok = providerMap["postgres"]
			}
			if !ok {
				logger.Warnf("No DBProvider found for database type '%s' (Datasource: %s). Skipping connection.", dbConfig.Type, name)
				continue
			}
		}

		// Get connection using the provider
		conn, err := provider.GetConnection(name)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get connection for '%s' using provider '%s': %w", name, provider.Type(), err)
		}
		
		// Once connection is established, create TxManager
		txManager := p.TxFactory.NewTransactionManager(conn)
		
		allConnections[name] = conn
		allTxManagers[name] = txManager
		logger.Debugf("Initialized DB Connection and TxManager for: %s (%s)", name, dbConfig.Type)
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

	return allConnections, allTxManagers, allProviders, nil
}

// NewMetadataTxManager extracts the "metadata" TransactionManager from the map.
func NewMetadataTxManager(allTxManagers map[string]tx.TransactionManager) (tx.TransactionManager, error) {
	tm, ok := allTxManagers["metadata"]
	if !ok {
		return nil, fmt.Errorf("metadata TransactionManager not found in aggregated map")
	}
	return tm, nil
}

// DefaultDBConnectionResolver implements adaptor.DBConnectionResolver
type DefaultDBConnectionResolver struct {
	providers map[string]adaptor.DBProvider
	cfg       *config.Config
}

// NewDefaultDBConnectionResolver creates a new DefaultDBConnectionResolver.
// To resolve circular dependencies, DBProviders are received as a list and converted to a map internally.
func NewDefaultDBConnectionResolver(p struct {
	fx.In
	DBProviders []adaptor.DBProvider `group:"db_providers"` // Receive as a list
	Cfg         *config.Config
}) adaptor.DBConnectionResolver {
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
	}
	return &DefaultDBConnectionResolver{
		providers: providerMap,
		cfg:       p.Cfg,
	}
}

// ResolveDBConnection resolves the database connection instance by name.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error) {
	dbConfig, ok := r.cfg.Surfin.Datasources[name]
	if !ok {
		return nil, fmt.Errorf("database configuration '%s' not found", name)
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

	// Get connection using the provider
	// The provider manages the connection pool internally and calls RefreshConnection as needed.
	conn, err := provider.GetConnection(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection '%s': %w", name, err)
	}
	
	// Expect GetConnection to return the latest valid connection
	// conn.RefreshConnection(ctx) // Optional: call RefreshConnection to ensure the connection is valid

	return conn, nil
}

// Module defines the application's Fx module.
// Add application-specific dependency injection settings here.
var Module = fx.Options(
	// DB Provider Modules
	gormadaptor.Module, // Only provides NewGormTransactionManagerFactory

	// Provide the aggregated map[string]adaptor.DBConnection and map[string]tx.TransactionManager
	// NewDBConnectionsAndTxManagers also provides map[string]adaptor.DBProvider
	fx.Provide(NewDBConnectionsAndTxManagers),
	
	// Provide the specific metadata TxManager required by JobFactory
	fx.Provide(fx.Annotate(
		NewMetadataTxManager,
		fx.ResultTags(`name:"metadata"`), // Named instance required by JobFactoryParams
	)),
	
	// Provide the concrete DBConnectionResolver implementation
	fx.Provide(fx.Annotate(
		NewDefaultDBConnectionResolver,
		fx.As(new(adaptor.DBConnectionResolver)),
	)),

	// Provide application migration FS by name
	fx.Provide(
		fx.Annotate(
			func(params struct {
				fx.In
				RawAppMigrationsFS embed.FS `name:"rawApplicationMigrationsFS"` // Raw embed.FS injected from main.go
			}) fs.FS {
				// Due to 'go:embed all:resources/migrations', the 'resources' directory is created at the root of the FS.
				// Remove this prefix so the framework can directly reference 'postgres' or 'mysql'.
				subFS, err := fs.Sub(params.RawAppMigrationsFS, "resources/migrations")
				if err != nil {
					// The go:embed path is fixed, so this should not normally be reached, but panic just in case.
					logger.Fatalf("Failed to create subdirectory for application migration FS: %v", err)
				}
				// Return fs.FS.
				return subFS
			},
			// Tag the result with the name 'weatherAppFS'.
			fx.ResultTags(`name:"weatherAppFS"`),
		),
	),
	
	// Provide the aggregated map[string]fs.FS
	fx.Provide(fx.Annotate(
		NewMigrationFSMap,
		fx.ResultTags(`name:"allMigrationFS"`),
	)),
	migrationfs.Module, // Explicitly provide framework migration FS on the application side
)
