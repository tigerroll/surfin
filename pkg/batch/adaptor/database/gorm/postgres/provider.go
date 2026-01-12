// Package postgres provides a GORM DBProvider implementation for PostgreSQL and Redshift databases.
package postgres

import (
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/postgres"
	gormadaptor "github.com/tigerroll/surfin/pkg/batch/adaptor/database/gorm"
	"gorm.io/gorm"
)

// init registers the PostgreSQL dialector factory with the gorm adaptor.
func init() {
	gormadaptor.RegisterDialector("postgres", func(cfg config.DatabaseConfig) (gorm.Dialector, error) {
		p := &gormadaptor.PostgresDBProvider{} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return postgres.Open(connStr), nil
	})
}

// NewProvider creates a new PostgreSQL DBProvider.
// This function is intended to be used with fx.Provide.
func NewProvider(cfg *config.Config) adaptor.DBProvider {
	return gormadaptor.NewPostgresProvider(cfg)
}
