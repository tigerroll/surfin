package gorm

import (
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"

	// Blank imports for GORM dialects to ensure they are registered.
	_ "gorm.io/driver/mysql"    // MySQL GORM driver
	_ "gorm.io/driver/postgres" // PostgreSQL GORM driver
	_ "gorm.io/driver/sqlite"   // SQLite GORM driver
)

// Module exports the components of the gorm adapter package for dependency injection (excluding concrete DB Providers).
var Module = fx.Options(
	fx.Provide(NewGormTransactionManagerFactory), // Provides the TransactionManagerFactory.
	fx.Provide(fx.Annotate( // Provides the GormDBConnectionResolver with specific annotations.
		NewGormDBConnectionResolver,
		fx.As(new(database.DBConnectionResolver)),
		fx.As(new(coreAdapter.ResourceConnectionResolver)), // Also provides as coreAdapter.ResourceConnectionResolver.
	)),
)
