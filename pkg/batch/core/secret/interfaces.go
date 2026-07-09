package secret

// SecretProvider defines the interface for resolving secrets from specific URI schemes.
type SecretProvider interface {
	// Supports returns true if the provider can resolve the given URI.
	Supports(uri string) bool
	// Resolve retrieves the secret value from the given URI.
	Resolve(uri string) (any, error)
}

// SecretResolver defines the interface for the central secret resolution service.
type SecretResolver interface {
	// Resolve resolves the secret value from the given URI.
	Resolve(uri string) (any, error)
}
