package gcs

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
	storageConfig "github.com/tigerroll/surfin/pkg/batch/adapter/storage/config"
	coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// gcsAdapter implements the storage.StorageAdapter interface, providing concrete operations for GCS.
type gcsAdapter struct {
	client *storage.Client
	cfg    storageConfig.StorageConfig
	name   string
}

// Verify that gcsAdapter implements the storage.StorageAdapter interface.
var _ storageAdapter.StorageAdapter = (*gcsAdapter)(nil)

// NewGCSAdapter creates a new gcsAdapter instance.
func NewGCSAdapter(ctx context.Context, cfg storageConfig.StorageConfig, name string) (storageAdapter.StorageAdapter, error) {
	var opts []option.ClientOption
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &gcsAdapter{
		client: client,
		cfg:    cfg,
		name:   name,
	}, nil
}

// Close closes the internal GCS client and releases associated resources.
func (a *gcsAdapter) Close() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// Type returns the type of the resource ("gcs").
func (a *gcsAdapter) Type() string {
	return a.cfg.Type
}

// Name returns the name of this connection.
func (a *gcsAdapter) Name() string {
	return a.name
}

// Upload uploads data to the specified bucket and object name.
func (a *gcsAdapter) Upload(ctx context.Context, bucket, objectName string, data io.Reader, contentType string) error {
	if bucket == "" {
		bucket = a.cfg.BucketName
	}
	if bucket == "" {
		return fmt.Errorf("bucket name is not specified in config or arguments for object '%s'", objectName)
	}

	wc := a.client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	if contentType != "" {
		wc.ContentType = contentType
	}

	if _, err := io.Copy(wc, data); err != nil {
		wc.CloseWithError(err) // Closes the writer with an error if one occurred during copy.
		return fmt.Errorf("failed to upload object '%s' to bucket '%s': %w", objectName, bucket, err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer for object '%s' in bucket '%s': %w", objectName, bucket, err)
	}
	logger.Debugf("Uploaded object '%s' to bucket '%s'.", objectName, bucket)
	return nil
}

// Download downloads data from the specified bucket and object name.
func (a *gcsAdapter) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	if bucket == "" {
		bucket = a.cfg.BucketName
	}
	if bucket == "" {
		return nil, fmt.Errorf("bucket name is not specified in config or arguments for object '%s'", objectName)
	}

	rc, err := a.client.Bucket(bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to download object '%s' from bucket '%s': %w", objectName, bucket, err)
	}
	logger.Debugf("Downloaded object '%s' from bucket '%s'.", objectName, bucket)
	return rc, nil
}

// ListObjects lists objects within the specified bucket and prefix, calling a callback function for each.
func (a *gcsAdapter) ListObjects(ctx context.Context, bucket, prefix string, fn func(objectName string) error) error {
	if bucket == "" {
		bucket = a.cfg.BucketName
	}
	if bucket == "" {
		return fmt.Errorf("bucket name is not specified in config or arguments for prefix '%s'", prefix)
	}

	it := a.client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list objects in bucket '%s' with prefix '%s': %w", bucket, prefix, err)
		}
		if err := fn(attrs.Name); err != nil {
			return fmt.Errorf("callback function failed for object '%s': %w", attrs.Name, err)
		}
	}
	logger.Debugf("Listed objects in bucket '%s' with prefix '%s'.", bucket, prefix)
	return nil
}

// DeleteObject deletes the specified object from the bucket.
func (a *gcsAdapter) DeleteObject(ctx context.Context, bucket, objectName string) error {
	if bucket == "" {
		bucket = a.cfg.BucketName
	}
	if bucket == "" {
		return fmt.Errorf("bucket name is not specified in config or arguments for object '%s'", objectName)
	}

	if err := a.client.Bucket(bucket).Object(objectName).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object '%s' from bucket '%s': %w", objectName, bucket, err)
	}
	logger.Debugf("Deleted object '%s' from bucket '%s'.", objectName, bucket)
	return nil
}

// Config returns the configuration used by this adapter.
func (a *gcsAdapter) Config() storageConfig.StorageConfig {
	return a.cfg
}

// GCSProvider implements the storage.StorageProvider interface, managing the lifecycle of GCS connections.
type GCSProvider struct {
	cfg         *coreConfig.Config
	connections map[string]storageAdapter.StorageAdapter
	mu          sync.RWMutex
}

// Verify that GCSProvider implements the storage.StorageProvider interface.
var _ storageAdapter.StorageProvider = (*GCSProvider)(nil)

// NewGCSProvider creates a new GCSProvider instance.
func NewGCSProvider(cfg *coreConfig.Config) storageAdapter.StorageProvider {
	return &GCSProvider{
		cfg:         cfg,
		connections: make(map[string]storageAdapter.StorageAdapter),
	}
}

// GetConnection retrieves a StorageAdapter connection with the specified name.
// It first attempts to return an existing connection from the cache. If no connection
// is found, it creates a new one based on the application configuration, caches it,
// and then returns it. This method uses double-checked locking to ensure thread-safe
// access and creation of connections.
func (p *GCSProvider) GetConnection(name string) (storageAdapter.StorageAdapter, error) {
	p.mu.RLock()
	conn, ok := p.connections[name]
	p.mu.RUnlock()
	if ok {
		return conn, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check locking
	conn, ok = p.connections[name]
	if ok {
		return conn, nil
	}

	// Create new connection
	var storageCfg storageConfig.StorageConfig
	rawAdapterConfig, ok := p.cfg.Surfin.AdapterConfigs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'adapter' configuration format: expected map[string]interface{}")
	}
	adapterConfig, ok := rawAdapterConfig["storage"]
	if !ok {
		return nil, fmt.Errorf("no 'storage' adapter configuration found in Surfin.AdapterConfigs")
	}

	storageConfigsMap, ok := adapterConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'storage' configuration format: expected map[string]interface{}")
	}

	namedConfig, ok := storageConfigsMap[name]
	if !ok {
		return nil, fmt.Errorf("storage configuration for name '%s' not found", name)
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Result:           &storageCfg,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToSliceHookFunc(","),
		),
		TagName: "yaml", // Specifies the YAML tag for decoding.
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder for storage config: %w", err)
	}
	if err := decoder.Decode(namedConfig); err != nil {
		return nil, fmt.Errorf("failed to decode storage config for '%s': %w", name, err)
	}

	if storageCfg.Type != p.Type() {
		return nil, fmt.Errorf("requested storage type '%s' does not match provider type '%s'", storageCfg.Type, p.Type())
	}

	conn, err = NewGCSAdapter(context.Background(), storageCfg, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS adapter for '%s': %w", name, err)
	}

	p.connections[name] = conn
	logger.Infof("Created new GCS connection '%s'.", name)
	return conn, nil
}

// CloseAll closes all connections managed by this provider.
func (p *GCSProvider) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for name, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close GCS connection '%s': %w", name, err))
		} else {
			logger.Infof("Closed GCS connection '%s'.", name)
		}
	}
	p.connections = make(map[string]storageAdapter.StorageAdapter) // Clears connections.
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while closing GCS connections: %v", errs)
	}
	return nil
}

// Type returns the type of resource handled by this provider ("gcs").
func (p *GCSProvider) Type() string {
	return "gcs"
}

// ForceReconnect forces the closure and re-establishment of an existing connection with the specified name.
func (p *GCSProvider) ForceReconnect(name string) (storageAdapter.StorageAdapter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.connections[name]; ok {
		if err := conn.Close(); err != nil {
			logger.Warnf("Failed to gracefully close GCS connection '%s' during force reconnect: %v", name, err)
		}
		delete(p.connections, name)
		logger.Infof("Closed existing GCS connection '%s' for force reconnect.", name)
	}

	// Re-create the connection
	return p.GetConnection(name)
}
