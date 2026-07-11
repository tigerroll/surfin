// Package main provides the entry point and dependency injection configuration for the hello-world example application.
// It defines the application's configuration, provides dummy implementations for infrastructure components
// (like databases and migrations), and registers the necessary batch components and listeners.
package main

import (
	"context"
	"io/fs"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/fx"

	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	database "github.com/tigerroll/surfin/pkg/batch/adapter/database"
	dummy "github.com/tigerroll/surfin/pkg/batch/adapter/database/dummy"
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	migration "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	decision "github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	split "github.com/tigerroll/surfin/pkg/batch/core/job/split"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	secret "github.com/tigerroll/surfin/pkg/batch/core/secret"
	incrementer "github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	inmemoryRepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/inmemory"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	appjob "github.com/tigerroll/surfin/example/hello-world/internal/app/job"
	apprunner "github.com/tigerroll/surfin/example/hello-world/internal/app/runner"
)

// dummyMigrator is a no-op implementation of the migration.Migrator interface.
// It performs no actual migration operations, as the hello-world application does not require a database.
type dummyMigrator struct{}

// Up performs no operation.
func (d *dummyMigrator) Up(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Up called, doing nothing.")
	return nil
}

// Down performs no operation.
func (d *dummyMigrator) Down(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Down called, doing nothing.")
	return nil
}

// Close performs no operation.
func (d *dummyMigrator) Close() error {
	logger.Debugf("Dummy Migrator: Close called, doing nothing.")
	return nil
}

// dummyMigratorProvider is a no-op implementation of the migration.MigratorProvider interface.
type dummyMigratorProvider struct{}

// NewMigrator returns a new dummy Migrator instance.
func (d *dummyMigratorProvider) NewMigrator(conn database.DBConnection) migration.Migrator {
	return &dummyMigrator{}
}

// dummySecretResolver is a no-op implementation of secret.SecretResolver.
type dummySecretResolver struct{}

// Ensure dummySecretResolver satisfies the secret.SecretResolver interface.
var _ secret.SecretResolver = (*dummySecretResolver)(nil)

// Resolve returns nil, nil as no secrets are resolved in this dummy implementation.
func (d *dummySecretResolver) Resolve(uri string) (any, error) {
	return nil, nil
}

// dummyMetricRecorder is a no-op implementation of the metrics.MetricRecorder interface.
type dummyMetricRecorder struct{}

func (d *dummyMetricRecorder) RecordJobStart(ctx context.Context, execution *model.JobExecution) {}
func (d *dummyMetricRecorder) RecordJobEnd(ctx context.Context, execution *model.JobExecution)   {}
func (d *dummyMetricRecorder) RecordChunkCommit(ctx context.Context, stepExecution *model.StepExecution, commitCount int64) {
}
func (d *dummyMetricRecorder) RecordDuration(ctx context.Context, name string, duration float64, attrs ...attribute.KeyValue) {
}
func (d *dummyMetricRecorder) RecordExecutionError(ctx context.Context, err error) {}
func (d *dummyMetricRecorder) RecordItemProcess(ctx context.Context, stepExecution *model.StepExecution, item int64) {
}
func (d *dummyMetricRecorder) RecordItemRead(ctx context.Context, stepExecution *model.StepExecution, item int64) {
}
func (d *dummyMetricRecorder) RecordItemRetry(ctx context.Context, stepExecution *model.StepExecution, err error) {
}
func (d *dummyMetricRecorder) RecordItemSkip(ctx context.Context, stepExecution *model.StepExecution, err error) {
}
func (d *dummyMetricRecorder) RecordItemWrite(ctx context.Context, stepExecution *model.StepExecution, item int64) {
}
func (d *dummyMetricRecorder) RecordStepStart(ctx context.Context, stepExecution *model.StepExecution) {
}
func (d *dummyMetricRecorder) RecordStepEnd(ctx context.Context, stepExecution *model.StepExecution) {
}

// dummyTracer is a no-op implementation of the metrics.Tracer interface.
type dummyTracer struct{}

// StartJobSpan returns the context and a no-op cleanup function.
func (d *dummyTracer) StartJobSpan(ctx context.Context, execution *model.JobExecution) (context.Context, func()) {
	return ctx, func() {}
}

// StartStepSpan returns the context and a no-op cleanup function.
func (d *dummyTracer) StartStepSpan(ctx context.Context, stepExecution *model.StepExecution) (context.Context, func()) {
	return ctx, func() {}
}

// RecordError performs no operation.
func (d *dummyTracer) RecordError(ctx context.Context, msg string, err error) {}

// RecordEvent performs no operation.
func (d *dummyTracer) RecordEvent(ctx context.Context, name string, attrs map[string]interface{}) {}

// dummyPortDBConnectionResolver is a no-op implementation of database.DBConnectionResolver
// and coreAdapter.ResourceConnectionResolver, used to satisfy dependencies in a DB-less environment.
type dummyPortDBConnectionResolver struct{}

// ResolveConnection returns a dummy ResourceConnection.
func (d *dummyPortDBConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveConnection called for '%s'.", name)
	return dummy.NewDummyDBConnection(name), nil
}

// ResolveConnectionName returns the default connection name.
func (d *dummyPortDBConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveConnectionName called, returning default '%v'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnectionName returns the default connection name.
func (d *dummyPortDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnectionName called, returning default '%s'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnection returns a dummy DBConnection instance.
func (d *dummyPortDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (database.DBConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnection called for '%s'.", name)
	return dummy.NewDummyDBConnection(name), nil
}

// realEnvironmentExpander implements the config.EnvironmentExpander interface
// by expanding environment variables using os.ExpandEnv.
type realEnvironmentExpander struct{}

// Expand expands environment variables in the input byte slice.
func (r *realEnvironmentExpander) Expand(input []byte) ([]byte, error) {
	expanded := os.ExpandEnv(string(input))
	logger.Debugf("Real EnvironmentExpander: Expanded '%s' to '%s'", string(input), expanded)
	return []byte(expanded), nil
}

// provideRealEnvironmentExpander provides a real implementation of config.EnvironmentExpander.
func provideRealEnvironmentExpander() config.EnvironmentExpander {
	return &realEnvironmentExpander{}
}

// GetApplicationOptions constructs and returns a slice of uber-fx options to configure the application's dependency injection graph.
func GetApplicationOptions(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes) []fx.Option {
	cfg, err := config.LoadConfig(envFilePath, embeddedConfig, &dummySecretResolver{})
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

	// Dummy providers to satisfy framework migration dependencies.
	options = append(options, fx.Provide(func() migration.MigratorProvider { return &dummyMigratorProvider{} }))
	options = append(options, fx.Provide(fx.Annotate(
		func() map[string]fs.FS { return make(map[string]fs.FS) },
		fx.ResultTags(`name:"allMigrationFS"`),
	)))

	// Add dummy provider for DBConnectionResolver.
	options = append(options, fx.Provide(fx.Annotate(
		func() *dummyPortDBConnectionResolver { return &dummyPortDBConnectionResolver{} },
		fx.As(new(database.DBConnectionResolver)),
		fx.As(new(coreAdapter.ResourceConnectionResolver)),
	)))

	// Provides a map of dummy DBProvider instances.
	options = append(options, fx.Provide(func() map[string]database.DBProvider {
		return map[string]database.DBProvider{
			"default":  dummy.NewDummyDBProvider(),
			"metadata": dummy.NewDummyDBProvider(),
			"dummy":    dummy.NewDummyDBProvider(),
		}
	}))

	// Add dummy providers for transaction manager-related dependencies.
	options = append(options, fx.Provide(func() tx.TransactionManagerFactory {
		return &dummy.DummyTxManagerFactory{}
	}))
	options = append(options, fx.Provide(func() map[string]tx.TransactionManager {
		return make(map[string]tx.TransactionManager)
	}))
	options = append(options, fx.Provide(fx.Annotate(dummy.NewMetadataTxManager, fx.ResultTags(`name:"metadata"`))))

	// Add real provider for config.EnvironmentExpander.
	options = append(options, fx.Provide(provideRealEnvironmentExpander))

	// Add dummy providers for metrics and tracing.
	options = append(options, fx.Provide(func() metrics.MetricRecorder { return &dummyMetricRecorder{} }))
	options = append(options, fx.Provide(func() metrics.Tracer { return &dummyTracer{} }))

	options = append(options, logger.Module)
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
	options = append(options, appjob.Module)

	return options
}
