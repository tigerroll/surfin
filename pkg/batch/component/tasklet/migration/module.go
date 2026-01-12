// Package migration provides the Fx module for the MigrationTasklet component.
// It handles database schema migrations within the batch framework.
package migration

import (
	"io/fs"

	"go.uber.org/fx"

	port "surfin/pkg/batch/core/application/port"
	config "surfin/pkg/batch/core/config"
	jsl "surfin/pkg/batch/core/config/jsl"
	support "surfin/pkg/batch/core/config/support"
	job "surfin/pkg/batch/core/domain/repository"
	"surfin/pkg/batch/support/util/logger"

	"surfin/pkg/batch/core/adaptor"
	"surfin/pkg/batch/component/tasklet/migration/drivers"
	"surfin/pkg/batch/core/tx"
)

// MigrationTaskletComponentBuilderParams defines the dependencies for NewMigrationTaskletComponentBuilder.
type MigrationTaskletComponentBuilderParams struct {
	fx.In
	AllDBConnections map[string]adaptor.DBConnection
	AllTxManagers    map[string]tx.TransactionManager
	AllMigrationFS   map[string]fs.FS `name:"allMigrationFS"`
	AllDBProviders   map[string]adaptor.DBProvider
}

// NewMigrationTaskletComponentBuilder creates a jsl.ComponentBuilder for MigrationTasklet.
// It receives its core dependencies (AllDBConnections, AllTxManagers, AllMigrationFS, AllDBProviders) via Fx.
func NewMigrationTaskletComponentBuilder(migratorProvider MigratorProvider, p MigrationTaskletComponentBuilderParams) jsl.ComponentBuilder {
	// Returns a builder function with the standard signature, called by JobFactory to construct the component.
	return func(
		cfg *config.Config,
		repo job.JobRepository,
		resolver port.ExpressionResolver,
		dbResolver port.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		return NewMigrationTasklet(cfg, repo, p.AllDBConnections, p.AllTxManagers, resolver, dbResolver, p.AllMigrationFS, properties, p.AllDBProviders, migratorProvider)
	}
}

// migrationTaskletBuilder is a struct to receive the migration tasklet builder via Fx.
type migrationTaskletBuilder struct {
	fx.In
	MigrationTaskletBuilder jsl.ComponentBuilder `name:"migrationTasklet"`
}

// RegisterMigrationTaskletBuilder registers the migration tasklet builder with the JobFactory.
func RegisterMigrationTaskletBuilder(jf *support.JobFactory, builders migrationTaskletBuilder) {
	jf.RegisterComponentBuilder("migrationTasklet", builders.MigrationTaskletBuilder)
	logger.Debugf("Component 'migrationTasklet' was registered with JobFactory.")
}

// Module provides the ComponentBuilder for [MigrationTasklet].
var Module = fx.Options(
	// Provide the concrete implementation for the MigratorProvider interface.
	fx.Provide(NewMigratorProvider),
	fx.Provide(fx.Annotate(
		NewMigrationTaskletComponentBuilder,
		fx.ResultTags(`name:"migrationTasklet"`),
	)),
	fx.Invoke(RegisterMigrationTaskletBuilder),
	drivers.Module, // Include the module that registers golang-migrate drivers
)
