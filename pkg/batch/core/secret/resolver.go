package secret

import (
	"fmt"
)

// CompositeSecretResolver manages multiple SecretProviders and delegates resolution to the appropriate one.
type CompositeSecretResolver struct {
	providers []SecretProvider
}

// NewCompositeSecretResolver creates a new CompositeSecretResolver.
// It accepts optional providers. If none are provided, it defaults to FileSecretProvider and EnvSecretProvider.
func NewCompositeSecretResolver(providers ...SecretProvider) *CompositeSecretResolver {
	if len(providers) == 0 {
		providers = []SecretProvider{
			&FileSecretProvider{},
			&EnvSecretProvider{},
		}
	}
	return &CompositeSecretResolver{
		providers: providers,
	}
}

// Resolve iterates through registered providers and returns the resolved secret from the first matching provider.
func (r *CompositeSecretResolver) Resolve(uri string) (any, error) {
	for _, p := range r.providers {
		if p.Supports(uri) {
			return p.Resolve(uri)
		}
	}
	return nil, fmt.Errorf("no provider found for uri: %s", uri)
}
