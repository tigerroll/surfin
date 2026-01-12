package filesystem

import (
	"embed"
	"io/fs"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

//go:embed resource
var rawFrameworkMigrationFS embed.FS

// ProvideFrameworkMigrationsFS embeds the framework migration files and returns them as fs.FS.
// It exposes the contents of the 'resource' directory directly.
func ProvideFrameworkMigrationsFS() fs.FS {
	subFS, err := fs.Sub(rawFrameworkMigrationFS, "resource")
	if err != nil {
		// This should not happen if 'resource' exists.
		logger.Fatalf("Failed to create subdirectory for framework migration FS: %v", err)
	}
	return subFS
}
