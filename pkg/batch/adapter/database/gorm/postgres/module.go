package postgres

import (
	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
)

// Module exports the PostgreSQL DBProvider for dependency injection.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewProvider,
			fx.As(new(adapter.DBProvider)),
			fx.ResultTags(`group:"`+adapter.DBProviderGroup+`"`),
		),
	),
)
