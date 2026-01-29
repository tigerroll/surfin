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

// init registers the SQLite dialector factory with the gorm adapter.
func init() {
	gormadapter.RegisterDialector("sqlite", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		if cfg.Database == "" {
			return nil, errors.New("sqlite database path cannot be empty")
		}
		p := &gormadapter.SQLiteDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return sqlite.Open(connStr), nil
	})
}

// NewProvider creates a new SQLite DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return gormadapter.NewSQLiteProvider(cfg)
}
