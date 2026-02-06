package sqlite

import (
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database" // Imports the database package.
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// Module exports the SQLite DBProvider for dependency injection.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewProvider,
			fx.As(new(database.DBProvider), new(coreAdapter.ResourceProvider)),
			fx.ResultTags(database.DBProviderGroup),
		),
	),
)
