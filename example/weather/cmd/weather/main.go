// Package main provides the entry point for the Weather application.
// It initializes the application's configuration, sets up dependencies,
// and starts the batch processing job.
package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	_ "embed"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/tigerroll/surfin/example/weather/internal/app"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/mysql"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/postgres"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database/gorm/sqlite"
	"github.com/tigerroll/surfin/pkg/batch/adapter/observability"
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage/gcs" // Imports the GCS module.
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3" // yaml package
)

// embeddedConfig embeds the content of the application's YAML configuration file (application.yaml).
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// applicationMigrationsFS is an embedded file system containing application-specific database migration files.
//
//go:embed all:resources/migrations
var applicationMigrationsFS embed.FS

// embeddedJSL embeds the content of the Job Specification Language (JSL) file (job.yaml) for the weather application.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// main is the entry point of the Weather batch application.
// It orchestrates the application's lifecycle, including startup, signal handling,
// and Fx container execution. This function initializes the application context
// and configuration, sets up a channel (`jobDoneChan`) for signaling job completion,
// and initiates the batch process. It also ensures graceful shutdown upon receiving OS signals.
func main() {
	// Create a cancellable context for the application lifecycle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling for graceful shutdown (e.g., Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel for JobCompletionSignaler.
	// This channel is used to notify the application externally about job completion.
	jobDoneChan := make(chan struct{})

	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal '%v'. Attempting to stop the job...", sig)
		cancel()
	}()

	// Get the path to the .env file from environment variables. Use ".env" as default if not set.
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

	// Define Fx options for adapter providers. This includes database, storage, web proxy, and metrics adapters.
	adapterProviderOptions := []fx.Option{
		// GORM-based database modules
		mysql.Module, // MySQL module
		postgres.Module,
		sqlite.Module,        // SQLite module
		gcs.Module,           // Adds the GCS storage adapter module.
		webproxy.Module,      // Adds the WebProxy adapter module.
		observability.Module, // Adds the Observability adapter module.
	}

	// Run the application.
	// Cast embeddedConfig and embeddedJSL to their respective type aliases and add jobDoneChan.
	app.RunApplication(ctx, envFilePath, config.EmbeddedConfig(embeddedConfig), jsl.JSLDefinitionBytes(embeddedJSL), applicationMigrationsFS, adapterProviderOptions, jobDoneChan)
	// Exit the process with exit code 0 after application execution completes.
	os.Exit(0)
}
