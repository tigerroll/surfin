package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "embed"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	jobRepo "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	
	"go.uber.org/fx"
)

// embeddedConfig はアプリケーションのYAML設定ファイルの内容を埋め込みます。
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// embeddedJSL はジョブ仕様言語 (JSL) ファイルの内容を埋め込みます。
//
//go:embed resources/job.yaml
var embeddedJSL []byte


// startJobExecution はアプリケーション起動時にジョブ実行を開始する Fx Hook ヘルパー関数です。
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

// onStopApplication はアプリケーションシャットダウンをログに記録する Fx Hook ヘルパー関数です。
func onStopApplication() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Application is shutting down.")
		return nil
	}
}

// main はアプリケーションのエントリポイントです。
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナルハンドリング (Ctrl+Cなど)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM) // シグナルハンドリング (Ctrl+Cなど)

	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal '%v'. Attempting to stop the job...", sig)
		cancel()
	}()

	envFilePath := os.Getenv("ENV_FILE_PATH")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	fxApp := fx.New(GetApplicationOptions(ctx, envFilePath, embeddedConfig, embeddedJSL)...) // GetApplicationOptions から返されたオプションを展開して fx.New に渡す
	fxApp.Run()
	if fxApp.Err() != nil {
		logger.Fatalf("Application run failed: %v", fxApp.Err())
	}
	os.Exit(0)
}
