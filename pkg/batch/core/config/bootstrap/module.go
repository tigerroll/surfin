package bootstrap

import (
	"go.uber.org/fx"
	"surfin/pkg/batch/core/support/expression"
	stepFactory "surfin/pkg/batch/engine/step/factory"
	partition "surfin/pkg/batch/engine/step/partition"
	"surfin/pkg/batch/engine/step/retry"
	"surfin/pkg/batch/engine/step/skip"
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
