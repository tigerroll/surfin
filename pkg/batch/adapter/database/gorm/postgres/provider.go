// Package postgres provides a GORM DBProvider implementation for PostgreSQL and Redshift databases.
package postgres

import (
	"fmt"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// init registers the PostgreSQL dialector factory with the GORM adapter.
// This function is automatically called when the package is imported.
// It allows the `gormadapter` to create PostgreSQL-specific `gorm.Dialector` instances
// based on the provided `dbconfig.DatabaseConfig`.
func init() {
	gormadapter.RegisterDialector("postgres", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		p := &PostgresDBProvider{BaseProvider: &gormadapter.BaseProvider{}} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return postgres.Open(connStr), nil
	})
}

// PostgresDBProvider implements adapter.DBProvider for PostgreSQL and Redshift connections.
type PostgresDBProvider struct {
	*gormadapter.BaseProvider
}

// ConnectionString generates the DSN (Data Source Name) for PostgreSQL connections.
//
// Parameters:
//
//	c: The `dbconfig.DatabaseConfig` containing connection details.
func (p *PostgresDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// Adjust to the DSN format expected by GORM (gorm.io/driver/postgres)
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.Sslmode)
}

// NewProvider creates a new `adapter.DBProvider` for PostgreSQL.
//
// This function is intended to be used with `fx.Provide` to register the PostgreSQL DBProvider
// in the application's dependency injection graph.
//
// Parameters:
//
//	cfg: The application's global configuration.
//
// Returns:
//
//	An `adapter.DBProvider` instance configured for PostgreSQL.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return &PostgresDBProvider{BaseProvider: gormadapter.NewBaseProvider(cfg, "postgres")}
}
