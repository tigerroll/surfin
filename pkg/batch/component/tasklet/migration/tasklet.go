package migration

import (
	"context"
	"io/fs"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// MigrationTasklet executes database migrations using provided fs.FS resources.
// It resolves the necessary database connection, migration file system, and migration table name, then executes the specified migration command ("up" or "down").
// After migration, it forces a re-connection to ensure the database connection is fresh and reflects any schema changes.
type MigrationTasklet struct {
	cfg            *config.Config
	allDBProviders map[string]database.DBProvider
	resolver       port.ExpressionResolver
	// dbResolver is the database connection resolver.
	dbResolver     database.DBConnectionResolver
	allMigrationFS map[string]fs.FS
	// properties are the raw properties passed from JSL.
	properties       map[string]string
	migratorProvider MigratorProvider

	// Configured properties from JSL.
	// dbConnectionName is the name of the database connection to use for migration.
	dbConnectionName string
	// fsName is the name of the `fs.FS` containing migration scripts.
	fsName string
	// migrationDir is the directory within the `fs.FS` where migration scripts are located.
	// If empty, it defaults to the database type.
	migrationDir string
	// command is the migration command to execute (e.g., "up", "down").
	command string
	// isFrameworkMigration indicates if this is a framework-level migration (influences table name).
	isFrameworkMigration bool

	// ec is the execution context for the tasklet.
	ec model.ExecutionContext
}

// NewMigrationTasklet creates a new MigrationTasklet instance.
//
// It initializes the tasklet with the provided configuration, repositories,
// database connections, transaction managers, resolvers, file systems,
// properties, and migrator provider. It extracts and validates
// necessary properties from the `properties` map.
//
// Parameters:
//
//	cfg: The application's global configuration.
//	resolver: The expression resolver for dynamic property resolution.
//	txFactory: The transaction manager factory (currently not directly used within this tasklet's logic, but passed for completeness).
//	dbResolver: The database connection resolver.
//	migratorProvider: The provider for obtaining Migrator instances.
//	allMigrationFS: A map of all registered migration file systems.
//	properties: A map of properties configured in JSL for this tasklet.
//	allDBProviders: A map of all registered DBProvider instances.
//
// Returns:
//
//	A new MigrationTasklet instance or an error if initialization fails.
func NewMigrationTasklet(
	cfg *config.Config,
	resolver port.ExpressionResolver,
	txFactory tx.TransactionManagerFactory, // This parameter is currently not used within the tasklet's logic.
	dbResolver database.DBConnectionResolver,
	migratorProvider MigratorProvider,
	allMigrationFS map[string]fs.FS,
	properties map[string]string,
	allDBProviders map[string]database.DBProvider,
) (*MigrationTasklet, error) {

	taskletName := "migration_tasklet"
	logger.Debugf("MigrationTasklet builder received properties: %v", properties)

	// Required properties from JSL.
	// Note: Using 'dbRef' for JSL compatibility, though 'dbConnectionName' is the internal field name.
	dbConnectionName, ok := properties["dbRef"]
	if !ok || dbConnectionName == "" {
		return nil, exception.NewBatchErrorf(taskletName, "Property 'dbRef' is required for MigrationTasklet")
	}

	// Note: Using 'migrationFSName' for JSL compatibility, though 'fsName' is the internal field name.
	fsName, ok := properties["migrationFSName"]
	if !ok || fsName == "" {
		return nil, exception.NewBatchErrorf(taskletName, "Property 'migrationFSName' is required for MigrationTasklet")
	}

	migrationDir, ok := properties["migrationDir"]
	if !ok || migrationDir == "" {
		// If 'migrationDir' is empty, it will be defaulted to the database type during execution.
		// It is recommended to explicitly specify it in JSL for clarity.
		logger.Warnf("Property 'migrationDir' is missing. It will be defaulted to the database type during execution.")
	}

	command := properties["command"]
	if command == "" { // Default command.
		command = "up"
	}

	isFrameworkMigration := false
	if isFrameworkStr, ok := properties["isFramework"]; ok {
		if strings.ToLower(isFrameworkStr) == "true" {
			isFrameworkMigration = true
		}
	}

	t := &MigrationTasklet{
		cfg:                  cfg,
		allDBProviders:       allDBProviders,
		resolver:             resolver,
		dbResolver:           dbResolver,
		allMigrationFS:       allMigrationFS,
		properties:           properties,
		migratorProvider:     migratorProvider,
		dbConnectionName:     dbConnectionName,
		fsName:               fsName,
		migrationDir:         migrationDir,
		command:              command,
		isFrameworkMigration: isFrameworkMigration,
		ec:                   model.NewExecutionContext(),
	}

	logger.Debugf("MigrationTasklet initialized: DB=%s, FS=%s, Dir=%s, Command=%s, IsFramework=%t", dbConnectionName, fsName, migrationDir, command, isFrameworkMigration)

	return t, nil
}

// Execute runs the database migration logic.
// It resolves the database connection, migration file system, and migration table name, then executes the specified migration command ("up" or "down").
//
// After migration, it forces a re-connection to ensure the database connection is fresh.
//
// Parameters:
//
//	ctx: The context for the operation.
//	stepExecution: The current StepExecution instance.
//
// Returns:
//
//	model.ExitStatus: The exit status of the tasklet (e.g., Completed, Failed).
//	error: An error if the migration fails.
func (t *MigrationTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	taskletName := "migration_tasklet"
	logger.Infof("Starting database migration for DB connection '%s' using FS '%s' and directory '%s' with command '%s'.",
		t.dbConnectionName, t.fsName, t.migrationDir, t.command)

	// 1. Get DB Configuration to determine DB Type
	var dbConfig dbconfig.DatabaseConfig
	rawAdapterConfig, ok := t.cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "invalid 'adapter' configuration format: expected map[string]interface{}")
	}
	dbConfigsMap, ok := rawAdapterConfig["database"].(map[string]interface{})
	if !ok {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "no 'database' adapter configuration found in Surfin.AdapterConfigs")
	}
	rawConfig, ok := dbConfigsMap[t.dbConnectionName]
	if !ok {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Database configuration '%s' not found in adapter.database configs", t.dbConnectionName)
	}
	if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Failed to decode database config for '%s': %w", t.dbConnectionName, err)
	}

	// If the database type is 'dummy', skip the migration process entirely.
	// This is for DB-less mode or testing where no actual database operations are expected.
	if strings.TrimSpace(strings.ToLower(dbConfig.Type)) == "dummy" {
		logger.Infof("MigrationTasklet: DB connection '%s' is configured as 'dummy'. Skipping migration.", t.dbConnectionName)
		return model.ExitStatusCompleted, nil
	}

	// 2. Get DBProvider for the specific database type
	provider, ok := t.allDBProviders[dbConfig.Type]
	if !ok {
		// Consider special cases like Redshift, which uses the PostgreSQL provider.
		if dbConfig.Type == "redshift" {
			provider, ok = t.allDBProviders["postgres"]
		}
		if !ok {
			return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "DBProvider for type '%s' not found", dbConfig.Type)
		}
	}

	// 3. Force reconnect to get a fresh, valid DBConnection for this migration.
	// This ensures that even if a previous operation closed the connection, we get a new one.
	dbConn, err := provider.ForceReconnect(t.dbConnectionName)
	if err != nil {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Failed to force reconnect DB connection '%s' before migration: %v", t.dbConnectionName, err)
	}

	// If the resolved DB connection is a dummy, skip actual migration.
	if dbConn.Type() == "dummy" {
		logger.Infof("MigrationTasklet: Skipping migration for dummy database connection '%s'.", t.dbConnectionName)
		return model.ExitStatusCompleted, nil
	}

	// 4. Resolve Migration FS
	migrationFS, ok := t.allMigrationFS[t.fsName]
	if !ok {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Migration FS '%s' not found", t.fsName)
	}

	// 5. Determine Migration Table Name
	migrationTable := FixedAppMigrationsTable
	if t.isFrameworkMigration {
		migrationTable = FixedFrameworkMigrationsTable
	}

	// 6. Determine Migration Directory (use DB type if not specified in JSL).
	migrationDir := t.migrationDir
	if migrationDir == "" {
		migrationDir = dbConn.Type()
		logger.Debugf("Using DB type '%s' as migration directory.", migrationDir)
	}

	// 7. Create Migrator instance
	migrator := t.migratorProvider.NewMigrator(dbConn)

	// 8. Execute Command
	switch t.command {
	case "up":
		if err := migrator.Up(ctx, migrationFS, migrationDir, migrationTable); err != nil {
			return model.ExitStatusFailed, exception.NewBatchError(taskletName, "Migration 'up' failed", err, false, false)
		}
	case "down":
		// Add support for 'down' command here if needed.
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Migration command '%s' is not yet supported", t.command)
	default:
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Unknown migration command: %s", t.command)
	}

	// 9. Ensure DB Connection is fresh after migration.
	// The migration tool might close the underlying connection, so force re-establishment.
	// This also ensures that schema changes are reflected in the connection.
	// Use the provider to force re-connect, which updates the connection in the provider's map.
	// Subsequent calls to DBConnectionResolver will then get the new connection.
	reconnectProvider, ok := t.allDBProviders[dbConfig.Type]
	if !ok {
		// Handle special cases like Redshift.
		if dbConfig.Type == "redshift" {
			reconnectProvider, ok = t.allDBProviders["postgres"]
		}
		if !ok {
			// If a provider for re-establishment is not found, this is a fatal error.
			return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Cannot find DBProvider for type '%s' to force reconnect after migration.", dbConfig.Type)
		}
	}

	// ForceReconnect closes the old connection, establishes a new one, and updates the internal map.
	_, err = reconnectProvider.ForceReconnect(t.dbConnectionName)
	if err != nil {
		// If re-establishment fails, it's a fatal error for subsequent operations.
		return model.ExitStatusFailed, exception.NewBatchError(taskletName, "Failed to force reconnect DB connection after migration", err, false, false)
	}

	// Return completed status on success.
	return model.ExitStatusCompleted, nil
}

// Close closes the `MigrationTasklet`.
//
// For this tasklet, there are no specific resources to close, so it always returns nil.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	An error if any resource closing fails (always nil for this tasklet).
func (t *MigrationTasklet) Close(ctx context.Context) error {
	return nil
}

// SetExecutionContext sets the `ExecutionContext` for the `MigrationTasklet`.
//
// Parameters:
//
//	ctx: The context for the operation.
//	ec: The ExecutionContext to set.
func (t *MigrationTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.ec = ec
	return nil
}

// GetExecutionContext retrieves the current `ExecutionContext` of the `MigrationTasklet`.
//
// Parameters:
//
//	ctx: The context for the operation.
//
// Returns:
//
//	The current ExecutionContext and an error if retrieval fails (always nil for this tasklet).
func (t *MigrationTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return t.ec, nil
}
