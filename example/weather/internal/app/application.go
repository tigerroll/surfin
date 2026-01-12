package app

import (
	"context"
	"embed"
	"time"

	model "surfin/pkg/batch/core/domain/model"
	config "surfin/pkg/batch/core/config"
	"surfin/pkg/batch/core/config/bootstrap"
	"surfin/pkg/batch/core/config/jsl"
	item "surfin/pkg/batch/component/item" // 4.4.1: core/item -> component/item
	"surfin/pkg/batch/core/job/decision"
	batchlistener "surfin/pkg/batch/listener" // 4.1.1: core/job/listener -> listener, 名前を統一
	"surfin/pkg/batch/core/job/split"
	usecase "surfin/pkg/batch/core/application/usecase" // 修正: launch -> core/application/usecase
	"surfin/pkg/batch/core/metrics"
	jobRepo "surfin/pkg/batch/core/domain/repository"
	supportConfig "surfin/pkg/batch/core/config/support"
	"surfin/pkg/batch/core/support/incrementer"
	tasklet "surfin/pkg/batch/component/tasklet/generic" // 4.4.2: core/tasklet -> component/tasklet/generic
	"surfin/pkg/batch/infrastructure/repository/sql"
	"surfin/pkg/batch/support/util/logger"
	
	migrationTasklet "surfin/pkg/batch/component/tasklet/migration"
	jobRunner "surfin/pkg/batch/core/job/runner" // 追加: JobRunner モジュールをインポート

	"go.uber.org/fx"
	
	// core "surfin/pkg/batch/core" // 削除

	appJob "surfin/example/weather/internal/job"
	weatherProcessor "surfin/example/weather/internal/step/processor"
	weatherReader "surfin/example/weather/internal/step/reader"
	weatherWriter "surfin/example/weather/internal/step/writer"
)

// RunApplication sets up and runs the batch application using uber-fx.
func RunApplication(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes, applicationMigrationsFS embed.FS, dbProviderOptions []fx.Option) {
	// Context setting and signal handling moved to main.go

	cfg, err := config.LoadConfig(envFilePath, embeddedConfig)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level based on loaded configuration
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

	// DB接続とTxManagerのオプション生成ロジックは、フレームワークコアに移動したため削除
	// dbOptions, err := database.CreateNamedDBConnectionAndTxManagerOptions(cfg)
	// if err != nil {
	// 	logger.Fatalf("Failed to generate named DB connection options: %v", err)
	// }

	app := fx.New(
		fx.Supply(
			embeddedConfig,
			embeddedJSL,
			fx.Annotate(envFilePath, fx.ResultTags(`name:"envFilePath"`)),
			fx.Annotate(
				applicationMigrationsFS,
				fx.ResultTags(`name:"rawApplicationMigrationsFS"`),
			),
			cfg,
			fx.Annotate(
				appCtx,
				fx.As(new(context.Context)),
				fx.ResultTags(`name:"appCtx"`),
			),
		),

		fx.Options(dbProviderOptions...), // main.go から渡された DB Provider オプションを使用
		logger.Module,
		config.Module,
		metrics.Module,

		// DB Providerの直接提供はapp.Moduleに集約されたため削除
		// fx.Provide(fx.Annotate(postgres.NewProvider, fx.ResultTags(database.DBProviderGroup))),
		// fx.Provide(fx.Annotate(mysql.NewProvider, fx.ResultTags(database.DBProviderGroup))),

		bootstrap.Module,
		
		// stepFactory.Module, // 削除: bootstrap.Moduleで提供済み
		fx.Provide(supportConfig.NewJobFactory),
		usecase.Module, // 修正: launch -> usecase
		
		// JobRepository の提供 (sql.NewJobRepository を使用)
		fx.Provide(fx.Annotate(
			sql.NewJobRepository,
			fx.As(new(jobRepo.JobRepository)),
		)),
		batchlistener.Module, // joblistenerとsteplistenerを統合
		decision.Module,
		split.Module,
		jobRunner.Module, // 追加: JobRunner の実装を提供

		weatherReader.Module,
		weatherProcessor.Module,
		weatherWriter.Module,
		incrementer.Module,
		item.Module, // 復活
		tasklet.Module,
		migrationTasklet.Module, // MigrationTaskletのコンポーネントモジュールを追加
		appJob.Module,
		Module, // DB Providerの集約を含む

		// Start the main application logic
		fx.Invoke(fx.Annotate(startJobExecution, fx.ParamTags(
			"", // lc fx.Lifecycle
			"", // shutdowner fx.Shutdowner
			"", // jobLauncher *usecase.SimpleJobLauncher (具体的な型)
			"", // jobRepository jobRepo.JobRepository
			"", // cfg *config.Config
			`name:"appCtx"`, // appCtx context.Context
		))),
	)

	// Execute the application
	app.Run()

	if app.Err() != nil {
		logger.Fatalf("Application run failed: %v", app.Err())
	}
}

// startJobExecution is invoked by Fx to begin the batch job execution.
func startJobExecution(
    lc fx.Lifecycle,
    shutdowner fx.Shutdowner,
    jobLauncher *usecase.SimpleJobLauncher, // Concrete type used
    jobRepository jobRepo.JobRepository,
    cfg *config.Config,
    appCtx context.Context,
) {
	lc.Append(fx.Hook{
		OnStart: onStartJobExecution(jobLauncher, jobRepository, cfg, shutdowner, appCtx),
		OnStop:  onStopApplication(),
	})
}

// onStartJobExecution is an Fx Hook helper function that starts job execution upon application startup.
func onStartJobExecution(
    jobLauncher *usecase.SimpleJobLauncher, // Concrete type used
    jobRepository jobRepo.JobRepository,
    cfg *config.Config,
    shutdowner fx.Shutdowner,
    appCtx context.Context,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Panic recovered in job execution: %v", r)
				}
				logger.Infof("Requesting application shutdown after job completion.")
				
				// JobExecution IDは Launch 成功後にしか得られないため、ここでは JobLauncher の Unregister は行わない
				
				if err := shutdowner.Shutdown(); err != nil {
					logger.Errorf("Failed to shutdown application: %v", err)
				}
			}()

			jobName := cfg.Surfin.Batch.JobName
			logger.Infof("Starting actual job execution for job '%s'...", jobName)

			jobParams := model.NewJobParameters()

			jobExecution, err := jobLauncher.Launch(appCtx, jobName, jobParams)
			if err != nil {
				logger.Errorf("Failed to launch job '%s': %v", jobName, err)
				return
			}
			logger.Infof("Job '%s' launched successfully. Execution ID: %s", jobName, jobExecution.ID)

			pollingInterval := time.Duration(cfg.Surfin.Batch.PollingIntervalSeconds) * time.Second
			if pollingInterval == 0 {
				pollingInterval = 5 * time.Second
			}
			logger.Infof("Monitoring job '%s' (Execution ID: %s) with polling interval %v...", jobName, jobExecution.ID, pollingInterval)

			for {
				select {
				case <-ctx.Done():
					logger.Warnf("Application context cancelled. Stopping monitoring for job '%s' (Execution ID: %s).", jobName, jobExecution.ID)
					
					// Contextキャンセル時、JobExecutionがまだ終了していない場合は、JobLauncherに停止を要求
					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(context.Background(), jobExecution.ID)
					if fetchErr == nil && !latestExecution.Status.IsFinished() {
						logger.Warnf("Job '%s' (Execution ID: %s) was running. Attempting graceful stop via JobOperator.", jobName, jobExecution.ID)
						// JobOperatorのStopロジックを模倣して CancelFunc を呼び出す
						if cancelFunc, ok := jobLauncher.GetCancelFunc(jobExecution.ID); ok {
							cancelFunc()
						}
					}
					return
				case <-time.After(pollingInterval):
					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(ctx, jobExecution.ID)
					if fetchErr != nil {
						logger.Errorf("Failed to fetch latest status for JobExecution (ID: %s): %v", jobExecution.ID, fetchErr)
						continue
					}

					if latestExecution.Status.IsFinished() {
						logger.Infof("Job '%s' (Execution ID: %s) finished with status: %s, ExitStatus: %s",
							jobName, latestExecution.ID, latestExecution.Status, latestExecution.ExitStatus)
						
						// JobRunnerのdeferで UnregisterCancelFunc が呼ばれるため、ここでは不要
						return
					}
					logger.Debugf("Job '%s' (Execution ID: %s) is still running. Current status: %s", jobName, latestExecution.ID, latestExecution.Status)
				}
			}
		}()
		return nil
	}
}

// onStopApplication is an Fx Hook helper function that logs application shutdown.
func onStopApplication() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Application is shutting down.")
		return nil
	}
}
