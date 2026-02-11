// main package is the entry point for the hello-world batch application.
// It sets up the Fx application, loads configurations, and manages the job execution lifecycle.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "embed"

	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	jobRepo "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
	"gopkg.in/yaml.v3" // yaml package
)

// embeddedConfig embeds the content of the application's YAML configuration file (application.yaml).
// This configuration is loaded at application startup.
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// embeddedJSL embeds the Job Specification Language (JSL) file (job.yaml), defining the batch job's structure and components.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// startJobExecution is an Fx Hook helper function that initiates job execution
// upon application startup. It registers OnStart and OnStop hooks with the Fx lifecycle.
//
// Parameters:
//
//	lc: The Fx lifecycle to register hooks with.
//	shutdowner: The Fx shutdowner to trigger application shutdown.
//	jobLauncher: The concrete SimpleJobLauncher instance responsible for launching jobs.
//	jobRepository: The repository for persisting and retrieving job metadata.
//	cfg: The overall application configuration.
//	appCtx: The root context for the application.
func startJobExecution(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	jobLauncher *usecase.SimpleJobLauncher,
	jobRepository jobRepo.JobRepository,
	cfg *config.Config,
	appCtx context.Context,
) {
	lc.Append(fx.Hook{
		OnStart: onStartJobExecution(jobLauncher, jobRepository, cfg, shutdowner, appCtx),
		OnStop:  onStopApplication(),
	})
}

// onStartJobExecution is an Fx Hook helper function that returns a function
// to be executed when the application starts. It launches the batch job
// and monitors its execution, triggering application shutdown upon completion.
//
// Parameters:
//
//	jobLauncher: The concrete SimpleJobLauncher instance responsible for launching jobs.
//	jobRepository: The repository for persisting and retrieving job metadata.
//	cfg: The overall application configuration.
//	shutdowner: The Fx shutdowner to trigger application shutdown.
//	appCtx: The root context for the application.
//
// Returns:
//
//	A function that returns an error, to be executed on application startup.
func onStartJobExecution(
	jobLauncher *usecase.SimpleJobLauncher,
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

					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(context.Background(), jobExecution.ID)
					if fetchErr == nil && !latestExecution.Status.IsFinished() {
						logger.Warnf("Job '%s' (Execution ID: %s) was running. Attempting graceful stop via JobOperator.", jobName, jobExecution.ID)
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

						return
					}
					logger.Debugf("Job '%s' (Execution ID: %s) is still running. Current status: %s", jobName, latestExecution.ID, latestExecution.Status)
				}
			}
		}()
		return nil
	}
}

// onStopApplication is an Fx Hook helper function that returns a function
// to be executed when the application stops. It logs the application shutdown event.
//
// Returns:
//
//	A function that returns an error, to be executed on application stop.
func onStopApplication() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Application is shutting down.")
		return nil
	}
}

// main is the entry point of the hello-world batch application.
// It sets up the application context, handles OS signals for graceful shutdown,
// loads configuration, and initializes and runs the Fx application.
//
// The application will execute the "helloWorldJob" defined in job.yaml.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals (e.g., Ctrl+C) for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal '%v'. Attempting to stop the job...", sig)
		cancel()
	}()

	// Create a channel for JobCompletionSignaler.
	// This channel is used to notify the application externally about job completion.
	jobDoneChan := make(chan struct{})

	// Determine the path to the .env file, defaulting to ".env" if not specified.
	envFilePath := os.Getenv("ENV_FILE_PATH")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	// To ensure Fx's internal logs reflect the desired settings,
	// the logging configuration is loaded early from application.yaml and applied to the logger before Fx initialization.
	cfg := config.NewConfig()
	if err := yaml.Unmarshal(embeddedConfig, cfg); err != nil {
		logger.Errorf("Failed to unmarshal embedded application config for early logger setup: %v", err)
	} else {
		logger.SetLogFormat(cfg.Surfin.System.Logging.Format)
		logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	}

	// Aggregate all Fx options into a temporary slice.
	var fxOptions []fx.Option
	fxOptions = append(fxOptions, fx.Provide(func() chan struct{} { return jobDoneChan })) // Provide jobDoneChan to Fx
	fxOptions = append(fxOptions, GetApplicationOptions(ctx, envFilePath, embeddedConfig, embeddedJSL)...)

	fxApp := fx.New(
		fx.Options(fxOptions...), // Spread the aggregated options slice into fx.Options
	)
	fxApp.Run()
	if fxApp.Err() != nil { // Check for errors during Fx application startup or execution.
		logger.Fatalf("Application run failed: %v", fxApp.Err())
	}
	os.Exit(0)
}
