// Package postgres provides a GORM DBProvider implementation for PostgreSQL and Redshift databases.
package postgres

import (
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// init registers the PostgreSQL dialector factory with the gorm adapter.
func init() {
	gormadapter.RegisterDialector("postgres", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		p := &gormadapter.PostgresDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return postgres.Open(connStr), nil
	})
}

// NewProvider creates a new PostgreSQL DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return gormadapter.NewPostgresProvider(cfg)
}
