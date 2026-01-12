// Package sqlite provides a GORM DBProvider implementation for SQLite databases.
package sqlite

import (
	"errors"
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/sqlite"
	gormadaptor "github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm"
	"gorm.io/gorm"
)

// init registers the SQLite dialector factory with the gorm adaptor.
func init() {
	gormadaptor.RegisterDialector("sqlite", func(cfg config.DatabaseConfig) (gorm.Dialector, error) {
		if cfg.Database == "" {
			return nil, errors.New("sqlite database path cannot be empty")
		}
		p := &gormadaptor.SQLiteDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return sqlite.Open(connStr), nil
	})
}

// NewProvider creates a new SQLite DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adaptor.DBProvider {
	return gormadaptor.NewSQLiteProvider(cfg)
}
