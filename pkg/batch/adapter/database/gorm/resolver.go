package gorm

import (
	"context"
	_ "database/sql" // Blank import to ensure database/sql package is included for its side effects (e.g., driver registration).
	"fmt"

	"github.com/mitchellh/mapstructure" // Imports the mapstructure package for decoding configurations.
	"go.uber.org/fx"                    // Imports the Fx package for dependency injection.

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config" // Imports the database configuration package.
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	config "github.com/tigerroll/surfin/pkg/batch/core/config" // Imports the core configuration package.
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// GormDBConnectionResolver is the GORM implementation of database.DBConnectionResolver.
type GormDBConnectionResolver struct {
	dbProviders map[string]database.DBProvider // A map of DBProviders, keyed by database type (e.g., "postgres", "mysql").
	cfg         *config.Config                 // The application's global configuration.
}

// NewGormDBConnectionResolver creates a new GormDBConnectionResolver.
// It receives dependencies using Fx's parameter struct.
//
// Parameters:
//
//	p: An Fx parameter struct containing a slice of DBProviders and the application Config.
//
// Returns:
//
//	A new GormDBConnectionResolver instance.
func NewGormDBConnectionResolver(p struct {
	fx.In
	DBProviders []database.DBProvider `group:"db_providers"` // All DBProviders provided by Fx as a slice.
	Cfg         *config.Config        // The application's global configuration.
}) *GormDBConnectionResolver {
	// Converts the received slice of DBProviders into a map for easier lookup.
	providerMap := make(map[string]database.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
	}

	return &GormDBConnectionResolver{
		dbProviders: providerMap,
		cfg:         p.Cfg,
	}
}

// ResolveDBConnection resolves a database connection with the specified name.
// It attempts to reconnect if the connection is closed or invalid.
//
// Parameters:
//
//	ctx: The context for the operation.
//	name: The name of the database connection to resolve.
//
// Returns:
//
//	The resolved database.DBConnection and an error if resolution or reconnection fails.
func (r *GormDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (database.DBConnection, error) {
	// 1. Get DB type from configuration.
	var dbConfig dbconfig.DatabaseConfig
	rawAdapterConfig, ok := r.cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("DBConnectionResolver: invalid 'adapter' configuration format: expected map[string]interface{}")
	}
	dbAdapterConfig, ok := rawAdapterConfig["database"]
	if !ok {
		return nil, fmt.Errorf("DBConnectionResolver: no 'database' adapter configuration found in Surfin.AdapterConfigs for connection '%s'", name)
	}
	dbConfigsMap, ok := dbAdapterConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("DBConnectionResolver: invalid 'database' adapter configuration format for connection '%s': expected map[string]interface{} but got %T", name, dbAdapterConfig)
	}
	rawConfig, ok := dbConfigsMap[name]
	if !ok {
		return nil, fmt.Errorf("DBConnectionResolver: database configuration '%s' not found under 'adapter.database' configs", name)
	}
	if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
		return nil, fmt.Errorf("DBConnectionResolver: failed to decode database config for '%s': %w", name, err)
	}

	// 2. Select the appropriate DBProvider.
	provider, ok := r.dbProviders[dbConfig.Type]
	if !ok {
		// Consider special cases like Redshift (uses PostgreSQL provider).
		if dbConfig.Type == "redshift" {
			provider, ok = r.dbProviders["postgres"]
		}
		if !ok {
			return nil, fmt.Errorf("DBConnectionResolver: DBProvider for type '%s' not found for connection '%s'", dbConfig.Type, name)
		}
	}

	// 3. Get connection from DBProvider.
	conn, err := provider.GetConnection(name)
	if err != nil {
		return nil, fmt.Errorf("DBConnectionResolver: Failed to get connection '%s': %w", name, err)
	}

	// 4. Check connection health and attempt to reconnect if necessary.
	sqlDB, getDBErr := conn.GetSQLDB()
	if getDBErr != nil {
		logger.Debugf("DBConnectionResolver: Failed to get underlying *sql.DB for connection '%s' (possibly a dummy connection): %v", name, getDBErr)
		return conn, nil // Return the connection as is if it's a dummy.
	}

	pingErr := sqlDB.PingContext(ctx)
	if pingErr != nil {
		logger.Warnf("DBConnectionResolver: Connection '%s' is invalid (%v). Attempting to reconnect.", name, pingErr)
		reconnectedConn, reconnectErr := provider.ForceReconnect(name)
		if reconnectErr != nil {
			return nil, fmt.Errorf("DBConnectionResolver: Failed to reconnect connection '%s': %w", name, reconnectErr)
		}
		logger.Infof("DBConnectionResolver: Successfully reconnected connection '%s'.", name)
		return reconnectedConn, nil
	}

	return conn, nil
}

// ResolveConnection is part of the coreAdapter.ResourceConnectionResolver interface.
// It is implemented by calling ResolveDBConnection.
//
// Parameters:
//
//	ctx: The context for the operation.
//	name: The name of the resource connection to resolve.
//
// Returns:
//
//	The resolved coreAdapter.ResourceConnection and an error if resolution fails.
func (r *GormDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	return r.ResolveDBConnection(ctx, name)
}

// ResolveConnectionName is part of the coreAdapter.ResourceConnectionResolver interface.
// It resolves the name of the resource connection based on the execution context.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution (may be nil).
//	stepExecution: The current StepExecution (may be nil).
//	defaultName: The default connection name to use if no dynamic resolution occurs.
//
// Returns:
//
//	The resolved connection name and an error if resolution fails.
func (r *GormDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	// This method only resolves the connection name and does not perform connection health checks.
	// Dynamic resolution logic based on jobExecution or stepExecution can be added here if needed.
	return defaultName, nil
}

// ResolveDBConnectionName is part of the database.DBConnectionResolver interface.
// It resolves the name of the database connection based on the execution context.
//
// Parameters:
//
//	ctx: The context for the operation.
//	jobExecution: The current JobExecution (may be nil).
//	stepExecution: The current StepExecution (may be nil).
//	defaultName: The default connection name to use if no dynamic resolution occurs.
//
// Returns:
//
//	The resolved connection name and an error if resolution fails.
func (r *GormDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	return r.ResolveConnectionName(ctx, jobExecution, stepExecution, defaultName)
}
