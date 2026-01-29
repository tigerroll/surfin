// Package migration provides the Fx module for the MigrationTasklet component.
// It handles database schema migrations within the batch framework.
package migration

import (
	"io/fs"

	"go.uber.org/fx"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration/drivers"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
)

// MigrationTaskletComponentBuilderParams defines the dependencies for NewMigrationTaskletComponentBuilder.
type MigrationTaskletComponentBuilderParams struct {
	fx.In
	TxFactory        tx.TransactionManagerFactory
	DBResolver       port.DBConnectionResolver
	MigratorProvider MigratorProvider
	AllMigrationFS   map[string]fs.FS `name:"allMigrationFS"`
	AllDBProviders   map[string]adapter.DBProvider
}

// NewMigrationTaskletComponentBuilder creates a jsl.ComponentBuilder for MigrationTasklet.
func NewMigrationTaskletComponentBuilder(p MigrationTaskletComponentBuilderParams) jsl.ComponentBuilder {
	// Returns a builder function that constructs the MigrationTasklet.
	return func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		dbResolver port.DBConnectionResolver, // This dbResolver is an argument to ComponentBuilder, not an Fx-injected one.
		properties map[string]string,
	) (interface{}, error) {
		return NewMigrationTasklet(
			cfg,
			resolver,
			p.TxFactory,
			p.DBResolver, // Use the Fx-injected DBResolver.
			p.MigratorProvider,
			p.AllMigrationFS,
			properties,
			p.AllDBProviders,
		)
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
