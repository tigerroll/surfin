// Package mysql provides a GORM DBProvider implementation for MySQL databases.
package mysql

import (
	"surfin/pkg/batch/core/adaptor"
	"surfin/pkg/batch/core/config"
	"gorm.io/driver/mysql"
	gormadaptor "surfin/pkg/batch/adaptor/database/gorm"
	"gorm.io/gorm"
)

// init registers the MySQL dialector factory with the gorm adaptor.
func init() {
	gormadaptor.RegisterDialector("mysql", func(cfg config.DatabaseConfig) (gorm.Dialector, error) {
		p := &gormadaptor.MySQLDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return mysql.Open(connStr), nil
	})
}

// NewProvider creates a new MySQL DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adaptor.DBProvider {
	return gormadaptor.NewMySQLProvider(cfg)
}
