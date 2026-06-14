// Package main serves as the entry point for the Weather batch application.
// It initializes the application configuration, sets up dependency injection via Fx,
// and orchestrates the batch job lifecycle.
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
	"github.com/tigerroll/surfin/pkg/batch/adapter/storage/gcs"
	"github.com/tigerroll/surfin/pkg/batch/adapter/webproxy"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
	"gopkg.in/yaml.v3"
)

// embeddedConfig embeds the application's YAML configuration file (application.yaml).
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// applicationMigrationsFS embeds the directory containing application-specific database migration files.
//
//go:embed all:resources/migrations
var applicationMigrationsFS embed.FS

// resourcesFS embeds all YAML files within the resources directory, allowing dynamic selection of job definitions.
//
//go:embed resources/*.yaml
var resourcesFS embed.FS

// main initializes the application context, handles OS signals for graceful shutdown,
// loads configurations, selects the appropriate JSL definition based on the SERVICE_NAME
// environment variable, and starts the Fx application container.
func main() {
	// Create a cancellable context for the application lifecycle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling for graceful shutdown (e.g., Ctrl+C).
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel for JobCompletionSignaler to notify the application externally about job completion.
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

	// Load logging configuration early from application.yaml to ensure Fx logs reflect the desired settings.
	cfg := config.NewConfig()
	if err := yaml.Unmarshal(embeddedConfig, cfg); err != nil {
		logger.Errorf("Failed to unmarshal embedded application config for early logger setup: %v", err)
	} else {
		logger.SetLogFormat(cfg.Surfin.System.Logging.Format)
		logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	}

	// Determine the JSL file name based on the SERVICE_NAME environment variable.
	serviceName := os.Getenv("SERVICE_NAME")
	jslFileName := "resources/job.yaml" // Default
	if serviceName != "" {
		jslFileName = "resources/" + serviceName + "_job.yaml"
		// Update JobName in config if SERVICE_NAME is provided.
		cfg.Surfin.Batch.JobName = serviceName + "Job"

		// Re-marshal the updated configuration to bytes.
		updatedConfigBytes, err := yaml.Marshal(cfg)
		if err != nil {
			logger.Fatalf("Failed to marshal updated config: %v", err)
		}
		embeddedConfig = updatedConfigBytes
	}

	// Load the JSL definition from the embedded filesystem.
	embeddedJSL, err := resourcesFS.ReadFile(jslFileName)
	if err != nil {
		logger.Fatalf("Failed to load JSL file '%s': %v", jslFileName, err)
	}

	// Define Fx options for adapter providers.
	adapterProviderOptions := []fx.Option{
		mysql.Module,
		postgres.Module,
		sqlite.Module,
		gcs.Module,
		webproxy.Module,
		observability.Module,
	}

	// Run the application.
	app.RunApplication(ctx, envFilePath, config.EmbeddedConfig(embeddedConfig), jsl.JSLDefinitionBytes(embeddedJSL), applicationMigrationsFS, adapterProviderOptions, jobDoneChan)

	// Exit the process with exit code 0 after application execution completes.
	os.Exit(0)
}
