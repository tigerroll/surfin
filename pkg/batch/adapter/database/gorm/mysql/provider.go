// Package mysql provides a GORM DBProvider implementation for MySQL databases.
package mysql

import (
	"fmt"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	gormadapter "github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// init registers the MySQL dialector factory with the GORM adapter.
// This function is automatically called when the package is imported.
// It allows the `gormadapter` to create MySQL-specific `gorm.Dialector` instances
// based on the provided `dbconfig.DatabaseConfig`.
func init() {
	gormadapter.RegisterDialector("mysql", func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
		p := &MySQLDBProvider{BaseProvider: &gormadapter.BaseProvider{}} // Creates a temporary instance to call the ConnectionString method.
		connStr := p.ConnectionString(cfg)
		return mysql.Open(connStr), nil
	})
}

// MySQLDBProvider implements adapter.DBProvider for MySQL connections.
type MySQLDBProvider struct {
	*gormadapter.BaseProvider
}

// ConnectionString generates the DSN (Data Source Name) for MySQL connections.
//
// Parameters:
//
//	c: The `dbconfig.DatabaseConfig` containing connection details.
func (p *MySQLDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// DSN format expected by GORM (gorm.io/driver/mysql)
	// user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	var authPart string
	if c.User != "" {
		authPart = c.User
		if c.Password != "" {
			authPart = fmt.Sprintf("%s:%s", c.User, c.Password)
		}
		authPart += "@" // Add @ if username is present
	}
	return fmt.Sprintf("%stcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		authPart, c.Host, c.Port, c.Database)
}

// NewProvider creates a new `adapter.DBProvider` for MySQL.
//
// This function is intended to be used with `fx.Provide` to register the MySQL DBProvider
// in the application's dependency injection graph.
//
// Parameters:
//
//	cfg: The application's global configuration.
//
// Returns:
//
//	An `adapter.DBProvider` instance configured for MySQL.
func NewProvider(cfg *config.Config) adapter.DBProvider {
	return &MySQLDBProvider{BaseProvider: gormadapter.NewBaseProvider(cfg, "mysql")}
}
