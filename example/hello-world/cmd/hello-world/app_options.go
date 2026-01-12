package main

import (
	"context"

	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	decision "github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	split "github.com/tigerroll/surfin/pkg/batch/core/job/split"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	incrementer "github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	inmemoryRepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/inmemory"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"

	appjob "github.com/tigerroll/surfin/example/hello-world/internal/app/job"
	apprunner "github.com/tigerroll/surfin/example/hello-world/internal/app/runner"
)

// GetApplicationOptions は uber-fx のオプションを構築し、スライスとして返します。
// この関数は fx.New の呼び出しの前に定義されている必要があります。
func GetApplicationOptions(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes) []fx.Option {
	cfg, err := config.LoadConfig(envFilePath, embeddedConfig)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

	var options []fx.Option

	options = append(options, fx.Supply(
		embeddedConfig,
		embeddedJSL,
		fx.Annotate(envFilePath, fx.ResultTags(`name:"envFilePath"`)),
		cfg,
		fx.Annotate(appCtx, fx.As(new(context.Context)), fx.ResultTags(`name:"appCtx"`)),
	))
	options = append(options, logger.Module)
	options = append(options, config.Module)
	options = append(options, metrics.Module)
	options = append(options, bootstrap.Module)
	options = append(options, fx.Provide(supportConfig.NewJobFactory))
	options = append(options, usecase.Module)
	options = append(options, inmemoryRepo.Module)
	options = append(options, batchlistener.Module)
	options = append(options, decision.Module)
	options = append(options, split.Module)
	options = append(options, apprunner.Module)
	options = append(options, incrementer.Module)
	options = append(options, item.Module)
	options = append(options, fx.Invoke(fx.Annotate(startJobExecution, fx.ParamTags("", "", "", "", "", `name:"appCtx"`))))
	options = append(options, helloTasklet.Module)
	options = append(options, appjob.Module) // アプリケーション固有の JobBuilder を提供するモジュールを直接追加

	return options
}
