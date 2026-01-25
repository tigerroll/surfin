package expression

import (
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"go.uber.org/fx"
)

// Module defines FX options related to ExpressionResolver.
var Module = fx.Options(
	// Provides DefaultExpressionResolver as the core.ExpressionResolver interface.
	fx.Provide(fx.Annotate(
		NewDefaultExpressionResolver,
		fx.As(new(port.ExpressionResolver)),
	)),
)
