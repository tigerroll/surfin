// Package inmemory provides an in-memory implementation of the JobRepository interface.
// This module integrates the in-memory repository into the application's dependency graph using Fx.
package inmemory

import (
	"go.uber.org/fx"

	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	dummy "github.com/tigerroll/surfin/pkg/batch/adaptor/database/dummy" // Import of dummy module.
)

// Module is an Fx module that provides InMemoryJobRepository as a repository.JobRepository interface.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewInMemoryJobRepository,
			fx.As(new(repository.JobRepository)),
		),
	),
	dummy.Module, // Automatically configure a dummy adapter when InMemoryJobRepository is being used.
)
