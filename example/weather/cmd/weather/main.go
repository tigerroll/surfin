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
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// embeddedConfig embeds the content of the application's YAML configuration file.
// This file is used to load configuration at application startup.
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// applicationMigrationsFS is an embedded file system containing database migration files.
// This bundles migration scripts into the application binary.
//
//go:embed all:resources/migrations
var applicationMigrationsFS embed.FS

// embeddedJSL embeds the content of the Job Specification Language (JSL) file.
// This file defines the flow and components of the batch job.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// getDBProviderOptions selects the DB Provider to use based on environment variables.
// If the "DB_ADAPTORS" environment variable is set, it selects DB Providers based on its comma-separated value.
// If not set, Postgres, MySQL, and SQLite are used by default.
//
// Returns:
//
//	A list of fx.Option to provide to the Fx application.
func getDBProviderOptions() []fx.Option {
	// Get the list of DB Providers to use from environment variables (e.g., "postgres,sqlite")
	adaptors := os.Getenv("DB_ADAPTORS")
	if adaptors == "" {
		// Use Postgres, MySQL, and SQLite as defaults (for compatibility)
		adaptors = "postgres,mysql,sqlite"
	}

	options := make([]fx.Option, 0)
	for _, adaptorName := range strings.Split(adaptors, ",") {
		adaptorName = strings.TrimSpace(adaptorName)
		if adaptorName == "" {
			continue
		}

		if provider, ok := app.DBProviderMap[adaptorName]; ok {
			// provider is of type func(cfg *config.Config) adaptor.DBProvider
			options = append(options, fx.Provide(fx.Annotate(provider, fx.ResultTags(`group:"`+adaptor.DBProviderGroup+`"`))))
			logger.Debugf("DB Provider '%s' selected and registered.", adaptorName)
		} else {
			logger.Warnf("DB Provider '%s' is configured but not recognized/supported. Skipping.", adaptorName)
		}
	}
	return options
}

// main is the entry point of the application.
// It manages the startup of the batch application, signal handling, and execution of the Fx container.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling for graceful shutdown (e.g., Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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

	// Run the application
	app.RunApplication(ctx, envFilePath, embeddedConfig, embeddedJSL, applicationMigrationsFS, dbProviderOptions)
	// Exit the process with exit code 0 after application execution completes
	os.Exit(0)
}
