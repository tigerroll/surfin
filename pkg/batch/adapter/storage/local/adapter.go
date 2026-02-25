// Package local provides a local file system implementation of the storage adapter interfaces.
package local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"

	storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	storageConfig "github.com/tigerroll/surfin/pkg/batch/adapter/storage/config"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

const (
	// ProviderType defines the type identifier for this local storage provider.
	ProviderType = "local"
)

// localAdapter implements the storage.StorageConnection interface for local file system operations.
type localAdapter struct {
	cfg  storageConfig.StorageConfig
	name string
}

// Verify that localAdapter implements the storage.StorageConnection interface.
var _ storageAdapter.StorageConnection = (*localAdapter)(nil)

// NewLocalAdapter creates a new localAdapter instance.
// It validates the BaseDir configuration and attempts to create it if it doesn't exist.
func NewLocalAdapter(cfg storageConfig.StorageConfig, name string) (storageAdapter.StorageConnection, error) {
	if cfg.BaseDir == "" {
		return nil, fmt.Errorf("local storage adapter '%s': BaseDir must be specified in configuration", name)
	}
	// Check if BaseDir exists and is a directory.
	info, err := os.Stat(cfg.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// If directory does not exist, try to create it.
			if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
				return nil, fmt.Errorf("local storage adapter '%s': failed to create BaseDir '%s': %w", name, cfg.BaseDir, err)
			}
		} else {
			return nil, fmt.Errorf("local storage adapter '%s': failed to stat BaseDir '%s': %w", name, cfg.BaseDir, err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("local storage adapter '%s': BaseDir '%s' is not a directory", name, cfg.BaseDir)
	}

	return &localAdapter{
		cfg:  cfg,
		name: name,
	}, nil
}

// Close does nothing for the local file system adapter as it holds no special resources.
func (a *localAdapter) Close() error {
	logger.Debugf("Local storage adapter '%s' closed.", a.name)
	return nil
}

// Type returns the type of the adapter, which is "local".
func (a *localAdapter) Type() string {
	return ProviderType
}

// Name returns the name of this connection.
func (a *localAdapter) Name() string {
	return a.name
}

// Upload uploads data to the specified bucket (treated as a directory) and object name (file path).
// It creates the necessary directories if they don't exist.
func (a *localAdapter) Upload(ctx context.Context, bucket, objectName string, data io.Reader, contentType string) error {
	fullPath, err := a.resolvePath(bucket, objectName)
	if err != nil {
		return fmt.Errorf("failed to resolve path for upload: %w", err)
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", fullPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write data to file '%s': %w", fullPath, err)
	}
	logger.Debugf("Uploaded data to '%s' (local adapter '%s').", fullPath, a.name)
	return nil
}

// Download downloads data from the specified bucket (treated as a directory) and object name (file path).
// The returned io.ReadCloser must be closed by the caller.
func (a *localAdapter) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	fullPath, err := a.resolvePath(bucket, objectName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path for download: %w", err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", fullPath, err)
	}
	logger.Debugf("Downloaded data from '%s' (local adapter '%s').", fullPath, a.name)
	return file, nil
}

// ListObjects lists objects within the specified bucket (treated as a directory) and prefix.
// It walks the directory tree and calls the provided function `fn` for each object found.
func (a *localAdapter) ListObjects(ctx context.Context, bucket, prefix string, fn func(objectName string) error) error {
	basePath, err := a.resolvePath(bucket, "") // Treat bucket as a directory
	if err != nil {
		return fmt.Errorf("failed to resolve base path for listing: %w", err)
	}

	// Ensure prefix is relative to BaseDir and join it.
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = filepath.Join(basePath, prefix)
	} else if prefix == "" {
		prefix = basePath
	}

	err = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil // Skip directories
		}

		// Filter by prefix
		if !strings.HasPrefix(path, prefix) {
			return nil
		}

		// Get object name relative to BaseDir
		objectName, err := filepath.Rel(basePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for '%s' from '%s': %w", path, basePath, err)
		}
		objectName = strings.ReplaceAll(objectName, "\\", "/") // Handle Windows paths

		if err := fn(objectName); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to list objects in '%s' with prefix '%s': %w", basePath, prefix, err)
	}
	logger.Debugf("Listed objects in '%s' with prefix '%s' (local adapter '%s').", basePath, prefix, a.name)
	return nil
}

// DeleteObject deletes the specified object from the bucket (treated as a directory).
// If the object does not exist, it logs a warning and returns nil.
func (a *localAdapter) DeleteObject(ctx context.Context, bucket, objectName string) error {
	fullPath, err := a.resolvePath(bucket, objectName)
	if err != nil {
		return fmt.Errorf("failed to resolve path for delete: %w", err)
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Attempted to delete non-existent object '%s' (local adapter '%s').", fullPath, a.name)
			return nil // Not an error if the file doesn't exist
		}
		return fmt.Errorf("failed to delete file '%s': %w", fullPath, err)
	}
	logger.Debugf("Deleted object '%s' (local adapter '%s').", fullPath, a.name)
	return nil
}

// Config returns the storage configuration used by this adapter.
func (a *localAdapter) Config() storageConfig.StorageConfig {
	return a.cfg
}

// resolvePath resolves the full path of a file relative to the BaseDir.
// It also performs a security check to ensure the resolved path does not escape the BaseDir.
func (a *localAdapter) resolvePath(bucket, objectName string) (string, error) {
	baseDir := a.cfg.BaseDir
	if baseDir == "" {
		return "", fmt.Errorf("BaseDir is not configured for local adapter '%s'", a.name)
	}

	if bucket == "" {
		bucket = a.cfg.BucketName // Use configured BucketName as default if not specified
	}

	var fullPath string
	if bucket == "" {
		// If no bucket is specified, place objectName directly under BaseDir
		fullPath = filepath.Join(baseDir, objectName)
	} else {
		// If bucket is specified, place objectName under BaseDir/bucket/objectName
		fullPath = filepath.Join(baseDir, bucket, objectName)
	}

	// Validate that the path does not escape BaseDir
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for BaseDir '%s': %w", baseDir, err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for '%s': %w", fullPath, err)
	}

	if !strings.HasPrefix(absFullPath, absBaseDir) {
		return "", fmt.Errorf("resolved path '%s' is outside of BaseDir '%s'", fullPath, baseDir)
	}

	return fullPath, nil
}

// LocalProvider implements the storage.StorageProvider interface for managing local file system connections.
type LocalProvider struct {
	cfg         *coreConfig.Config
	connections map[string]storageAdapter.StorageConnection
	mu          sync.RWMutex
}

// NewLocalProvider creates a new LocalProvider instance.
func NewLocalProvider(cfg *coreConfig.Config) storageAdapter.StorageProvider {
	return &LocalProvider{
		cfg:         cfg,
		connections: make(map[string]storageAdapter.StorageConnection),
	}
}

// GetConnection retrieves a StorageConnection connection by the given name.
// It creates a new connection if one does not already exist for the given name.
func (p *LocalProvider) GetConnection(name string) (storageAdapter.StorageConnection, error) {
	p.mu.RLock()
	conn, ok := p.connections[name]
	p.mu.RUnlock()
	if ok {
		return conn, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring lock
	conn, ok = p.connections[name]
	if ok {
		return conn, nil
	}

	// Decode StorageConfig from the application configuration.
	// The AdapterConfigs field is an interface{}, so type assertion is needed.
	rawAdapterConfig, ok := p.cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'adapter' configuration format: expected map[string]interface{}")
	}
	storageConfigMap, ok := rawAdapterConfig["storage"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'storage' configuration format: expected map[string]interface{}")
	}
	namedConfig, ok := storageConfigMap[name]
	if !ok {
		return nil, fmt.Errorf("storage configuration for name '%s' not found", name)
	}

	var storageCfg storageConfig.StorageConfig
	// Use mapstructure.DecoderConfig to recognize yaml tags.
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &storageCfg,
		TagName:  "yaml", // Specify the yaml tag here.
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder for storage config '%s': %w", name, err)
	}
	if err := decoder.Decode(namedConfig); err != nil {
		return nil, fmt.Errorf("failed to decode storage config for '%s': %w", name, err)
	}

	if storageCfg.Type != ProviderType {
		return nil, fmt.Errorf("storage config type mismatch for '%s': expected '%s', got '%s'", name, ProviderType, storageCfg.Type)
	}

	newConn, err := NewLocalAdapter(storageCfg, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create local adapter for '%s': %w", name, err)
	}

	p.connections[name] = newConn
	logger.Debugf("Created new local storage connection '%s'.", name)
	return newConn, nil
}

// CloseAll closes all connections managed by this provider.
func (p *LocalProvider) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for name, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close local storage connection '%s': %w", name, err))
		}
		delete(p.connections, name)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing local storage connections: %v", errs)
	}
	logger.Debugf("All local storage connections closed.")
	return nil
}

// Type returns the type of resource handled by this provider, which is "local".
func (p *LocalProvider) Type() string {
	return ProviderType
}

// ForceReconnect forces the closure and re-establishment of an existing connection with the specified name.
func (p *LocalProvider) ForceReconnect(name string) (storageAdapter.StorageConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.connections[name]; ok {
		if err := conn.Close(); err != nil {
			logger.Warnf("Failed to gracefully close local storage connection '%s' during force reconnect: %v", name, err)
		}
		delete(p.connections, name)
	}

	logger.Debugf("Forcing reconnect for local storage connection '%s'.", name)
	return p.GetConnection(name) // GetConnection will create a new one
}

// LocalConnectionResolver implements the storage.StorageConnectionResolver interface and manages multiple StorageProviders.
type LocalConnectionResolver struct {
	providers map[string]storageAdapter.StorageProvider
	cfg       *coreConfig.Config
}

// NewLocalConnectionResolver creates a new LocalConnectionResolver instance.
// It now accepts a map of providers, which is typically provided by NewStorageConnections.
func NewLocalConnectionResolver(providers map[string]storageAdapter.StorageProvider, cfg *coreConfig.Config) storageAdapter.StorageConnectionResolver {
	// The input 'providers' is already a map, so no need to convert.
	return &LocalConnectionResolver{
		providers: providers, // Use the map directly
		cfg:       cfg,
	}
}

// ResolveConnection resolves a generic resource connection by name.
func (r *LocalConnectionResolver) ResolveConnection(ctx context.Context, name string) (coreAdapter.ResourceConnection, error) {
	return r.ResolveStorageConnection(ctx, name)
}

// ResolveConnectionName resolves a generic resource connection name based on the execution context.
// Currently, it does not implement dynamic resolution logic and returns the default name.
// Logic to determine the connection name from jobExecution or stepExecution can be added if needed.
func (r *LocalConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	logger.Debugf("Resolving storage connection name. Defaulting to '%s'.", defaultName)
	return defaultName, nil
}

// ResolveStorageConnection resolves a StorageConnection connection instance by the given name.
// This method is responsible for ensuring that the returned connection is valid and re-established if necessary.
func (r *LocalConnectionResolver) ResolveStorageConnection(ctx context.Context, name string) (storageAdapter.StorageConnection, error) {
	// Determine the connection type from the configuration.
	// The AdapterConfigs field is an interface{}, so type assertion is needed.
	rawAdapterConfig, ok := r.cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'adapter' configuration format: expected map[string]interface{}")
	}
	storageConfigMap, ok := rawAdapterConfig["storage"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'storage' configuration format: expected map[string]interface{}")
	}
	namedConfig, ok := storageConfigMap[name]
	if !ok {
		return nil, fmt.Errorf("storage connection '%s' not found in configuration", name)
	}

	var tempCfg struct {
		Type string `yaml:"type"` // Use yaml tag for decoding.
	}
	if err := mapstructure.Decode(namedConfig, &tempCfg); err != nil {
		return nil, fmt.Errorf("failed to decode storage type for '%s': %w", name, err)
	}

	providerType := tempCfg.Type

	provider, ok := r.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("no storage provider found for type '%s' (connection '%s')", providerType, name)
	}

	conn, err := provider.GetConnection(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage connection '%s' from provider '%s': %w", name, providerType, err)
	}
	return conn, nil
}

// ResolveStorageConnectionName resolves the name of the data storage connection based on the execution context.
// This method applies the same logic as ResolveConnectionName.
func (r *LocalConnectionResolver) ResolveStorageConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
	return r.ResolveConnectionName(ctx, jobExecution, stepExecution, defaultName)
}
