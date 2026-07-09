package secret

import "go.uber.org/fx"

// Module provides the secret management components to the fx application.
var Module = fx.Module("secret",
	fx.Provide(
		// Register providers into the "secret_providers" group
		fx.Annotate(
			func() SecretProvider { return &FileSecretProvider{} },
			fx.ResultTags(`group:"secret_providers"`),
		),
		fx.Annotate(
			func() SecretProvider { return &EnvSecretProvider{} },
			fx.ResultTags(`group:"secret_providers"`),
		),
		// Provide the resolver, injecting the group
		fx.Annotate(
			func(providers []SecretProvider) *CompositeSecretResolver {
				return NewCompositeSecretResolver(providers...)
			},
			fx.ParamTags(`group:"secret_providers"`),
		),
		// Bind the interface
		func(r *CompositeSecretResolver) SecretResolver { return r },
	),
)
