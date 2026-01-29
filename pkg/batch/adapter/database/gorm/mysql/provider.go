// Package mysql provides a GORM DBProvider implementation for MySQL databases.
package mysql

import (
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// init registers the MySQL dialector factory with the gorm adapter.
func init() {
	gormadapter.RegisterDialector("mysql", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		p := &gormadapter.MySQLDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return mysql.Open(connStr), nil
	})
}

// NewProvider creates a new MySQL DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return gormadapter.NewMySQLProvider(cfg)
}
