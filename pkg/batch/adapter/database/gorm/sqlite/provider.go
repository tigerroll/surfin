// Package sqlite provides a GORM DBProvider implementation for SQLite databases.
package sqlite

import (
	"errors"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// init registers the SQLite dialector factory with the GORM adapter.
// This function is automatically called when the package is imported.
// It allows the `gormadapter` to create SQLite-specific `gorm.Dialector` instances
// based on the provided `dbconfig.DatabaseConfig`.
func init() {
	gormadapter.RegisterDialector("sqlite", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		if cfg.Database == "" { // Ensure database path is provided.
			return nil, errors.New("SQLite database path cannot be empty")
		}
		p := &SQLiteDBProvider{BaseProvider: &gormadapter.BaseProvider{}} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return sqlite.Open(connStr), nil
	})
}

// SQLiteDBProvider implements adapter.DBProvider for SQLite connections.
type SQLiteDBProvider struct {
	*gormadapter.BaseProvider
}

// ConnectionString generates the DSN (Data Source Name) for SQLite connections.
//
// Parameters:
//
//	c: The `dbconfig.DatabaseConfig` containing connection details.
func (p *SQLiteDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// GORM SQLite Dialector expects the file path directly
	return c.Database
}

// NewProvider creates a new `adapter.DBProvider` for SQLite.
//
// This function is intended to be used with `fx.Provide` to register the SQLite DBProvider
// in the application's dependency injection graph.
//
// Parameters:
//
//	cfg: The application's global configuration.
//
// Returns:
//
//	An `adapter.DBProvider` instance configured for SQLite.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return &SQLiteDBProvider{BaseProvider: gormadapter.NewBaseProvider(cfg, "sqlite")}
}
