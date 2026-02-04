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
	// TxFactory is the TransactionManagerFactory for creating transaction managers.
	TxFactory tx.TransactionManagerFactory
	// DBResolver is the DBConnectionResolver for resolving database connections.
	DBResolver adapter.DBConnectionResolver
	// MigratorProvider is the provider for obtaining Migrator instances.
	MigratorProvider MigratorProvider
	// AllMigrationFS is a map of all registered migration file systems, keyed by name.
	AllMigrationFS map[string]fs.FS `name:"allMigrationFS"`
	// AllDBProviders is a map of all registered DBProvider instances, keyed by database type.
	AllDBProviders map[string]adapter.DBProvider
}

// NewMigrationTaskletComponentBuilder creates a `jsl.ComponentBuilder` for `MigrationTasklet`.
//
// This builder function is responsible for instantiating `MigrationTasklet` with its required dependencies.
//
// Parameters:
//
//	p: The MigrationTaskletComponentBuilderParams containing all necessary dependencies.
//
// Returns:
//
//	A `jsl.ComponentBuilder` function that can create `MigrationTasklet` instances.
func NewMigrationTaskletComponentBuilder(p MigrationTaskletComponentBuilderParams) jsl.ComponentBuilder {
	return func(
		cfg *config.Config,
		resolver port.ExpressionResolver,
		dbResolver adapter.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		return NewMigrationTasklet(
			cfg,
			resolver,
			p.TxFactory,
			p.DBResolver, // Use the Fx-injected DBConnectionResolver.
			p.MigratorProvider,
			p.AllMigrationFS,
			properties,
			p.AllDBProviders,
		)
	}
}

// migrationTaskletBuilder is a struct to receive the migration tasklet builder via Fx.
// It is used internally by Fx to inject the named component builder.
type migrationTaskletBuilder struct {
	fx.In
	MigrationTaskletBuilder jsl.ComponentBuilder `name:"migrationTasklet"`
}

// RegisterMigrationTaskletBuilder registers the `MigrationTasklet` component builder with the `JobFactory`.
//
// This allows the framework to locate and use the `MigrationTasklet` when it's referenced in JSL (Job Specification Language) files.
// The component is registered under the name "migrationTasklet".
//
// Parameters:
//
//	jf: The JobFactory instance.
//	builders: A struct containing the `MigrationTasklet` component builder.
func RegisterMigrationTaskletBuilder(jf *support.JobFactory, builders migrationTaskletBuilder) {
	jf.RegisterComponentBuilder("migrationTasklet", builders.MigrationTaskletBuilder)
	logger.Debugf("Component 'migrationTasklet' was registered with JobFactory.")
}

// Module provides the `ComponentBuilder` for `MigrationTasklet` and registers it with the `JobFactory`.
var Module = fx.Options(
	// Provide the concrete implementation for the MigratorProvider interface.
	fx.Provide(NewMigratorProvider),
	fx.Provide(fx.Annotate(
		NewMigrationTaskletComponentBuilder,
		fx.ResultTags(`name:"migrationTasklet"`),
	)),
	fx.Invoke(RegisterMigrationTaskletBuilder), drivers.Module, // Include the module that registers golang-migrate drivers
)
