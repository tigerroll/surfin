package migration

import (
	"context"
	"io/fs"
	"strings"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// MigrationTasklet executes database migrations using provided fs.FS resources.
type MigrationTasklet struct {
	cfg              *config.Config
	repo             repository.JobRepository
	allDBConnections map[string]adaptor.DBConnection
	allDBProviders   map[string]adaptor.DBProvider
	resolver         port.ExpressionResolver
	dbResolver       port.DBConnectionResolver
	allMigrationFS   map[string]fs.FS
	properties       map[string]string
	migratorProvider MigratorProvider

	// Configured properties
	dbConnectionName     string
	fsName               string
	migrationDir         string
	command              string // e.g., "up", "down", "status".
	isFrameworkMigration bool   // Indicates if it's a framework migration.

	// Execution Context
	ec model.ExecutionContext
}

// NewMigrationTasklet creates a new MigrationTasklet instance.
func NewMigrationTasklet(
	cfg *config.Config,
	repo repository.JobRepository,
	allDBConnections map[string]adaptor.DBConnection,
	allTxManagers map[string]tx.TransactionManager,
	resolver port.ExpressionResolver,
	dbResolver port.DBConnectionResolver,
	allMigrationFS map[string]fs.FS,
	properties map[string]string,
	allDBProviders map[string]adaptor.DBProvider,
	migratorProvider MigratorProvider,
) (*MigrationTasklet, error) {

	taskletName := "migration_tasklet"
	logger.Debugf("MigrationTasklet builder received properties: %v", properties)

	// Required properties from JSL

	dbConnectionName, ok := properties["dbRef"] // Using old key 'dbRef' for JobFactory compatibility.
	if !ok || dbConnectionName == "" {
		return nil, exception.NewBatchErrorf(taskletName, "Property 'dbRef' is required for MigrationTasklet")
	}

	fsName, ok := properties["migrationFSName"] // Using old key 'migrationFSName' for JobFactory compatibility.
	if !ok || fsName == "" {
		return nil, exception.NewBatchErrorf(taskletName, "Property 'migrationFSName' is required for MigrationTasklet")
	}

	migrationDir, ok := properties["migrationDir"]
	if !ok || migrationDir == "" {
		// If migrationDir is empty, use the DB type as default.
		// However, it is recommended to explicitly specify it in JSL.
		logger.Warnf("Property 'migrationDir' is missing. Attempting to use DB type as default.")
		// Not checking here as DB connection might not be established yet. Will check in Execute.
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
		repo:                 repo,
		allDBConnections:     allDBConnections,
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
func (t *MigrationTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	taskletName := "migration_tasklet"
	logger.Infof("Starting database migration for DB connection '%s' using FS '%s' and directory '%s' with command '%s'.",
		t.dbConnectionName, t.fsName, t.migrationDir, t.command)

	// 1. Get DB Configuration to determine DB Type
	dbConfig, ok := t.cfg.Surfin.Datasources[t.dbConnectionName]
	if !ok {
		return model.ExitStatusFailed, exception.NewBatchErrorf(taskletName, "Database configuration '%s' not found", t.dbConnectionName)
	}

	// 2. Get DBProvider for the specific database type
	provider, ok := t.allDBProviders[dbConfig.Type]
	if !ok {
		// Consider special cases like Redshift.
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

	// 4. Determine Migration Table Name
	migrationTable := FixedAppMigrationsTable
	if t.isFrameworkMigration {
		migrationTable = FixedFrameworkMigrationsTable
	}

	// 5. Determine Migration Directory (use DB type if not specified in JSL).
	migrationDir := t.migrationDir
	if migrationDir == "" {
		migrationDir = dbConn.Type()
		logger.Debugf("Using DB type '%s' as migration directory.", migrationDir)
	}

	// 6. Create Migrator instance
	migrator := t.migratorProvider.NewMigrator(dbConn)

	// 7. Execute Command
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

	// 8. Refresh DB Connection (refresh connection after migration).
	refreshErr := dbConn.RefreshConnection(ctx)
	if refreshErr != nil {
		// Connection is likely closed.
		logger.Warnf("MigrationTasklet: Failed to refresh DB connection '%s' after migration: %v. Attempting to reconnect...", t.dbConnectionName, refreshErr)

		// Force re-establishment of connection using DBProvider.
		provider, ok := t.allDBProviders[dbConn.Type()]
		if !ok {
			// For Redshift, try Postgres Provider.
			dbType := dbConn.Type()
			if dbType == "redshift" {
				provider, ok = t.allDBProviders["postgres"]
			}
		}

		if ok {
			// ForceReconnect is defined in the DBProvider interface.
			_, err := provider.ForceReconnect(t.dbConnectionName)
			if err != nil {
				logger.Errorf("MigrationTasklet: Failed to force reconnect DB connection '%s': %v", t.dbConnectionName, err)
				// Reconnection failure is not fatal, but JobRepository updates are likely to fail.
			} else {
				// newConn refers to the same instance as the existing dbConn, and its internal state is updated.
				logger.Infof("MigrationTasklet: Successfully re-established DB connection '%s'.", t.dbConnectionName)
			}
		} else {
			logger.Errorf("MigrationTasklet: Cannot find DBProvider for type '%s' to force reconnect.", dbConn.Type())
		}
	}

	// If successful
	return model.ExitStatusCompleted, nil
}

// Close is the method for releasing resources.
func (t *MigrationTasklet) Close(ctx context.Context) error {
	// No resources to close in this simple tasklet
	return nil
}

// SetExecutionContext sets the ExecutionContext.
func (t *MigrationTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.ec = ec
	return nil
}

// GetExecutionContext retrieves the ExecutionContext.
func (t *MigrationTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return t.ec, nil
}
