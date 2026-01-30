package gorm

import (
	"go.uber.org/fx"

	// Blank imports for GORM dialects to ensure they are registered.
	_ "gorm.io/driver/mysql"    // MySQL GORM driver
	_ "gorm.io/driver/postgres" // PostgreSQL GORM driver
	_ "gorm.io/driver/sqlite"   // SQLite GORM driver
)

// Module exports the components of the gorm adapter package for dependency injection (excluding concrete DB Providers).
var Module = fx.Options(
	fx.Provide(NewGormTransactionManagerFactory), // Add TransactionManagerFactory provider
)
