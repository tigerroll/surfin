package expression

import (
	port "surfin/pkg/batch/core/application/port"
	"go.uber.org/fx"
)

// Module defines FX options related to ExpressionResolver.
var Module = fx.Options(
	// Provides DefaultExpressionResolver as the core.ExpressionResolver interface.
	fx.Provide(fx.Annotate(
		NewDefaultExpressionResolver,
		fx.As(new(port.ExpressionResolver)),
	)),
	// Provides DefaultDBConnectionResolver as the core.DBConnectionResolver interface.
	fx.Provide(fx.Annotate(
		NewDefaultDBConnectionResolver, // Dependency on ExpressionResolver is resolved automatically.
		fx.As(new(port.DBConnectionResolver)),
	)),
)
