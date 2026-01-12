package bootstrap

import (
	"go.uber.org/fx"
	"github.com/tigerroll/surfin/pkg/batch/core/support/expression"
	stepFactory "github.com/tigerroll/surfin/pkg/batch/engine/step/factory"
	partition "github.com/tigerroll/surfin/pkg/batch/engine/step/partition"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/retry"
	"github.com/tigerroll/surfin/pkg/batch/engine/step/skip"
)

// Module provides initializer-related components to Fx.
var Module = fx.Options(
	fx.Provide(NewBatchInitializer), // Defined in initializer.go
	fx.Invoke(LoadJSLDefinitionsHook), // Defined in initializer.go
	fx.Invoke(ApplyLoggingConfigHook), // Defined in initializer.go (Corresponds to A_LOG_1)
	
	expression.Module, // Provides Expression Resolver
	
	// Engine Components (Provides Step Factory, StepExecutor, Retry Module, Skip Module)
	stepFactory.Module,
	partition.Module,
	retry.Module,
	skip.Module,
)
