package migration

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	// Import database drivers for migrate
	// The following blank imports were removed as they were moved to their respective module files.
	// _ "github.com/golang-migrate/migrate/v4/database/mysql"
	// _ "github.com/golang-migrate/migrate/v4/database/sqlite"
)

// migratorImpl implements Migrator
type migratorImpl struct {
	dbConn adapter.DBConnection
	dbType string
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(dbConn adapter.DBConnection) Migrator {
	return &migratorImpl{
		dbConn: dbConn,
		dbType: dbConn.Type(),
	}
}

// getDatabaseDriver retrieves a migrate/v4 Driver based on the database type.
func (m *migratorImpl) getDatabaseDriver(sqlDB *sql.DB, tableName string) (database.Driver, error) {
	switch m.dbType {
	case "postgres", "redshift":
		// PostgreSQL/Redshift driver
		return postgres.WithInstance(sqlDB, &postgres.Config{
			MigrationsTable: tableName,
		})
	case "mysql":
		// MySQL driver
		return mysql.WithInstance(sqlDB, &mysql.Config{
			MigrationsTable: tableName,
		})
	case "sqlite":
		// SQLite driver
		return sqlite.WithInstance(sqlDB, &sqlite.Config{
			MigrationsTable: tableName,
		})
	default:
		return nil, fmt.Errorf("unsupported database type for migration: %s", m.dbType)
	}
}

func (m *migratorImpl) getMigrateInstance(migrationFS fs.FS, path string, tableName string) (*migrate.Migrate, error) {
	// 1. Get the underlying *sql.DB connection
	// Get *sql.DB from DBConnection.
	sqlDB, err := m.dbConn.GetSQLDB() // FIX: Use method from DBConnection interface
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 2. Create the source driver (iofs)
	sourceDriver, err := iofs.New(migrationFS, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create iofs source driver for path %s: %w", path, err)
	}

	// 3. Create the database driver instance
	dbDriver, err := m.getDatabaseDriver(sqlDB, tableName) // Use getDatabaseDriver.
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// 4. Create the migrate instance using the configured driver
	mInstance, err := migrate.NewWithInstance("iofs", sourceDriver, m.dbType, dbDriver) // FIX: Pass dbDriver.
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	return mInstance, nil
}

func (m *migratorImpl) runMigration(ctx context.Context, migrationFS fs.FS, path string, command string, tableName string) error {
	logger.Infof("Executing migration '%s' (Path: %s, Table: %s)", command, path, tableName)

	mInstance, err := m.getMigrateInstance(migrationFS, path, tableName)
	if err != nil {
		return fmt.Errorf("failed to get migrate instance: %w", err)
	}
	defer mInstance.Close()

	var migrateErr error

	if command == "up" {
		migrateErr = mInstance.Up() // Apply all pending migrations.
	} else if command == "down" {
		// Add support for 'down' command here if needed.
		return fmt.Errorf("unsupported migration command: %s", command)
	} else {
		return fmt.Errorf("unsupported migration command: %s", command)
	}

	if migrateErr != nil && migrateErr != migrate.ErrNoChange {
		// Check for version error separately (optional, but good practice).
		_, _, versionErr := mInstance.Version() // FIX: Receive three return values.
		if versionErr != nil {
			logger.Errorf("Migration failed and failed to retrieve version: %v", versionErr)
		}
		return fmt.Errorf("migration failed for command '%s' (DB: %s, Path: %s): %w", command, m.dbType, path, migrateErr)
	}

	logger.Infof("Migration '%s' completed successfully.", command)
	return nil
}

func (m *migratorImpl) Up(ctx context.Context, migrationFS fs.FS, path string, tableName string) error {
	return m.runMigration(ctx, migrationFS, path, "up", tableName)
}

func (m *migratorImpl) Down(ctx context.Context, migrationFS fs.FS, path string, tableName string) error {
	return m.runMigration(ctx, migrationFS, path, "down", tableName)
}

func (m *migratorImpl) Close() error {
	// golang-migrate instance is closed in runMigration defer, nothing to close here.
	return nil
}
