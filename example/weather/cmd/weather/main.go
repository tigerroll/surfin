package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "embed"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/tigerroll/surfin/example/weather/internal/app"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// embeddedConfig embeds the content of the application's YAML configuration file (application.yaml).
// This is used for loading configuration at application startup.
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// applicationMigrationsFS is an embedded file system containing database migration files.
// This bundles migration scripts into the application binary.
//
//go:embed all:resources/migrations
var applicationMigrationsFS embed.FS

// embeddedJSL embeds the content of the Job Specification Language (JSL) file (job.yaml).
// This file defines the flow and components of the batch job.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// getDBProviderOptions selects the DB Provider to use based on environment variables.
// If the "DB_ADAPTERS" environment variable is set, it selects DB Providers based on its comma-separated value.
// If not set, Postgres, MySQL, and SQLite are used by default.
//
// Returns:
//
//	A list of fx.Option to provide to the Fx application.
func getDBProviderOptions() []fx.Option {
	// Get the list of DB Providers to use from environment variables (e.g., "postgres,sqlite")
	adapters := os.Getenv("DB_ADAPTERS")
	if adapters == "" {
		// Use Postgres, MySQL, and SQLite as defaults (for compatibility)
		adapters = "postgres,mysql,sqlite"
	}

	options := make([]fx.Option, 0)
	for _, adapterName := range strings.Split(adapters, ",") {
		adapterName = strings.TrimSpace(adapterName)
		if adapterName == "" {
			continue
		}

		if provider, ok := app.DBProviderMap[adapterName]; ok {
			// provider is of type func(cfg *config.Config) adapter.DBProvider
			options = append(options, fx.Provide(fx.Annotate(provider, fx.ResultTags(`group:"`+adapter.DBProviderGroup+`"`))))
			logger.Debugf("DB Provider '%s' selected and registered.", adapterName)
		} else {
			logger.Warnf("DB Provider '%s' is configured but not recognized/supported. Skipping.", adapterName)
		}
	}
	return options
}

// main is the entry point of the application.
// It manages the startup of the batch application, signal handling, and execution of the Fx container.
// This function sets up a channel (`jobDoneChan`) for signaling job completion, initializes the
// application context and configuration, and starts the batch process.
func main() {
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

	// Provide selected DB Providers to Fx
	dbProviderOptions := getDBProviderOptions()

	// Run the application.
	// Cast embeddedConfig and embeddedJSL to their respective type aliases and add jobDoneChan.
	app.RunApplication(ctx, envFilePath, config.EmbeddedConfig(embeddedConfig), jsl.JSLDefinitionBytes(embeddedJSL), applicationMigrationsFS, dbProviderOptions, jobDoneChan)
	// Exit the process with exit code 0 after application execution completes.
	os.Exit(0)
}
