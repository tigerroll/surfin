package filesystem

import (
	"go.uber.org/fx"
)

// FrameworkMigrationsFSTag is the Fx tag for the embedded framework migrations filesystem.
const FrameworkMigrationsFSTag = `name:"frameworkMigrationsFS"`

// Module provides migration-related components used by Tasklet.
var Module = fx.Options(
	// Add a provider for the embedded framework migrations FS
	fx.Provide(fx.Annotate(
		ProvideFrameworkMigrationsFS,
		fx.ResultTags(FrameworkMigrationsFSTag),
	)),
)
