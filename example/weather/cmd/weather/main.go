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
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// embeddedConfig embeds the content of the application's YAML configuration file (application.yaml). It is used for loading configuration at application startup.
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// applicationMigrationsFS is an embedded file system containing database migration files. This bundles migration scripts into the application binary, allowing them to be applied at runtime without external file dependencies.
//
//go:embed all:resources/migrations
var applicationMigrationsFS embed.FS

// embeddedJSL embeds the content of the Job Specification Language (JSL) file (job.yaml). This file defines the flow and components of the batch job.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// main is the entry point of the application. It manages the startup of the batch application, signal handling, and execution of the Fx container.
// This function sets up a channel (`jobDoneChan`) for signaling job completion, initializes the application context and configuration, and starts the batch process.
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

	// Define Fx options for database providers. All GORM-based providers are included here.
	dbProviderOptions := []fx.Option{
		// gormmodule.Module is removed as NewGormTransactionManagerFactory is already provided in internal/app/module.go.
		mysql.Module,
		postgres.Module,
		sqlite.Module, // SQLite module
	}

	// Run the application.
	// Cast embeddedConfig and embeddedJSL to their respective type aliases and add jobDoneChan.
	app.RunApplication(ctx, envFilePath, config.EmbeddedConfig(embeddedConfig), jsl.JSLDefinitionBytes(embeddedJSL), applicationMigrationsFS, dbProviderOptions, jobDoneChan)
	// Exit the process with exit code 0 after application execution completes.
	os.Exit(0)
}
