package drivers

import (
	"go.uber.org/fx"

	// Blank imports for golang-migrate database drivers to ensure they are registered.
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
)

// Module provides the blank imports for golang-migrate database drivers.
// This ensures that the drivers are registered with the golang-migrate library
// when this Fx module is included in the application graph.
var Module = fx.Options() // fx.Options() is used to encapsuate blank imports.
