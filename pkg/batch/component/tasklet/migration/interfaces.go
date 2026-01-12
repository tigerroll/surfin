package migration

import (
	"context"
	"io/fs"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
)

// Fixed table names for migration tracking
const FixedFrameworkMigrationsTable = "batch_framework_migrations"
const FixedAppMigrationsTable = "batch_app_migrations"

// Migrator handles database schema migrations.
type Migrator interface {
	// Up applies all pending migrations.
	// tableName: The name of the table used to track migration history (e.g., batch_framework_migrations).
	Up(ctx context.Context, migrationFS fs.FS, path string, tableName string) error
	// Down rolls back all applied migrations.
	// tableName: The name of the table used to track migration history.
	Down(ctx context.Context, migrationFS fs.FS, path string, tableName string) error
	// Close releases resources used by the migrator.
	Close() error
}

// MigratorProvider is a factory for creating Migrator instances.
// This interface breaks the import cycle between 'adaptor/database' and 'component/tasklet/migration'.
type MigratorProvider interface {
	NewMigrator(dbConn adaptor.DBConnection) Migrator
}
