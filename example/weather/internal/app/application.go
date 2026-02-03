package app

import (
	"context"
	"embed"
	"time"

	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	tasklet "github.com/tigerroll/surfin/pkg/batch/component/tasklet/generic"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	jobRepo "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	"github.com/tigerroll/surfin/pkg/batch/core/job/split"
	"github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	"github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/sql"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	migrationTasklet "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	jobRunner "github.com/tigerroll/surfin/pkg/batch/core/job/runner"

	"go.uber.org/fx"

	appJob "github.com/tigerroll/surfin/example/weather/internal/job"
	weatherProcessor "github.com/tigerroll/surfin/example/weather/internal/step/processor"
	weatherReader "github.com/tigerroll/surfin/example/weather/internal/step/reader"
	weatherWriter "github.com/tigerroll/surfin/example/weather/internal/step/writer"
)

// RunApplication sets up and runs the batch application using uber-fx.
func RunApplication(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes, applicationMigrationsFS embed.FS, dbProviderOptions []fx.Option, jobDoneChan chan struct{}) {
	// Context setting and signal handling moved to main.go

	cfg, err := config.LoadConfig(envFilePath, embeddedConfig)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level based on loaded configuration
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

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
		fx.Provide(func() chan struct{} { return jobDoneChan }),

		fx.Options(dbProviderOptions...),
		logger.Module,
		config.Module,
		metrics.Module,

		bootstrap.Module,

		fx.Provide(supportConfig.NewJobFactory),
		usecase.Module,

		// Provide JobRepository (using sql.NewJobRepository)
		fx.Provide(fx.Annotate(
			sql.NewJobRepository,
			fx.As(new(jobRepo.JobRepository)),
		)),
		batchlistener.Module,
		decision.Module,
		split.Module,
		jobRunner.Module,

		weatherReader.Module,
		weatherProcessor.Module,
		weatherWriter.Module,
		incrementer.Module,
		item.Module,
		tasklet.Module,
		migrationTasklet.Module,
		appJob.Module,
		Module,

		// Start the main application logic
		fx.Invoke(fx.Annotate(startJobExecution, fx.ParamTags(
			"",              // lc fx.Lifecycle
			"",              // shutdowner fx.Shutdowner
			"",              // jobLauncher *usecase.SimpleJobLauncher (concrete type)
			"",              // jobRepository jobRepo.JobRepository
			"",              // cfg *config.Config
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

				// JobExecution ID is only available after successful Launch, so Unregister is not performed here.

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

					// If context is cancelled and JobExecution is not yet finished, request stop via JobLauncher.
					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(context.Background(), jobExecution.ID)
					if fetchErr == nil && !latestExecution.Status.IsFinished() {
						logger.Warnf("Job '%s' (Execution ID: %s) was running. Attempting graceful stop via JobOperator.", jobName, jobExecution.ID)
						// Mimic JobOperator's Stop logic by calling CancelFunc.
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

						// UnregisterCancelFunc is called by JobRunner's defer, so it's not needed here.
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
