package gorm

import (
	"go.uber.org/fx"

	// Blank imports for GORM dialects to ensure they are registered.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// Module exports the components of the gorm adapter package for dependency injection (excluding concrete DB Providers).
var Module = fx.Options(
	fx.Provide(NewGormTransactionManagerFactory), // Add TransactionManagerFactory provider
)
