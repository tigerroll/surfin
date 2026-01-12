package migration

import (
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
)

// migratorProviderImpl implements MigratorProvider
type migratorProviderImpl struct{}

// NewMigratorProvider creates a new MigratorProvider.
func NewMigratorProvider() MigratorProvider {
	return &migratorProviderImpl{}
}

// NewMigrator creates a new migration.Migrator instance by calling the existing NewMigrator function.
func (p *migratorProviderImpl) NewMigrator(dbConn adaptor.DBConnection) Migrator {
	return NewMigrator(dbConn)
}
