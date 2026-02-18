# タスクチケット: GCSストレージアダプターの実装

## 概要

このタスクは、バッチフレームワークにGoogle Cloud Storage (GCS) アダプターを実装し、GCSへのデータアクセスを可能にすることを目的とします。`docs/strategy/gcs_storage_adapter_design.md`
に定義された設計に基づき、共通インターフェースの定義、GCS固有の実装、および **`weather` アプリケーション**への統合を行います。
**本タスクで実装されるGCSアダプターは、汎用的な `StorageAdapter` インターフェースを実装するため、バッチフレームワークのアイテムリーダーやアイテムライターといったコアコンポーネントは、GCSに特化することなく、この抽象化されたインターフェースを通じてGCSと連携できます。これにより、将来的にローカルファイルシステムや他のクラウドストレージ向けのアダプターが追加された場合でも、同じリーダー/ライターのロジックを再利用することが可能になります。**

## タスクリスト

### タスク 1: 共通インターフェースと設定構造体の定義

#### 1.1. `pkg/batch/adapter/storage/interfaces.go` の作成

*   **目的**: 汎用的なストレージ操作を定義するインターフェース (`StorageAdapter`, `StorageProvider`, `StorageConnectionResolver`) を作成します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/interfaces.go` (新規作成)
*   **変更内容**:
    ```go
    package storage

    import (
    	"context"
    	"io"

    	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
    	storageConfig "github.com/tigerroll/surfin/pkg/batch/adapter/storage/config"
    )

    // StorageExecutor は汎用的なストレージ操作を定義します。
    // StorageAdapter に埋め込まれ、具体的なストレージ操作を提供します。
    type StorageExecutor interface {
    	Upload(ctx context.Context, bucket, objectName string, data io.Reader, contentType string) error
    	Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error)
    	ListObjects(ctx context.Context, bucket, prefix string, fn func(objectName string) error) error
    	DeleteObject(ctx context.Context, bucket, objectName string) error
    }

    // StorageAdapter は汎用的なデータストレージ接続を表します。
    // coreAdapter.ResourceConnection と StorageExecutor を埋め込み、リソース接続としての機能と具体的なストレージ操作を提供します。
    type StorageAdapter interface {
    	coreAdapter.ResourceConnection // Close(), Type(), Name() を継承
    	StorageExecutor                // Upload(), Download(), ListObjects(), DeleteObject() を継承

    	Config() storageConfig.StorageConfig
    }

    // StorageProvider はデータストレージ接続の取得と管理を行います。
    // coreAdapter.ResourceProvider と同様の機能を提供しますが、直接埋め込みは行いません。
    // 注意: coreAdapter.ResourceProvider には Name() メソッドがありますが、StorageProvider には定義されていません。
    // これは、StorageProvider が特定の「タイプ」（例: "storage"）のプロバイダであり、そのタイプ自体が識別子として機能するためです。
    // ComponentBuilder などで coreAdapter.ResourceProvider として扱う場合は、別途ラッパーなどが必要になる可能性があります。
    type StorageProvider interface {
    	// GetConnection は指定された名前の StorageAdapter 接続を取得します。
    	GetConnection(name string) (StorageAdapter, error)
    	// CloseAll はこのプロバイダが管理する全ての接続を閉じます。
    	CloseAll() error
    	// Type はこのプロバイダが扱うリソースのタイプ（例: "database", "storage"）を返します。
    	Type() string
    	// ForceReconnect は指定された名前の既存の接続を強制的に閉じ、再確立します。
    	ForceReconnect(name string) (StorageAdapter, error)
    }

    // StorageConnectionResolver は実行コンテキストに基づいて適切なデータストレージ接続インスタンスを解決します。
    // coreAdapter.ResourceConnectionResolver を埋め込み、汎用的なリソース解決機能を提供します。
    type StorageConnectionResolver interface {
    	coreAdapter.ResourceConnectionResolver // ResolveConnection(), ResolveConnectionName() を継承

    	// ResolveStorageConnection は指定された名前の StorageAdapter 接続インスタンスを解決します。
    	// 返される接続が有効であり、必要に応じて再確立されることを保証します。
    	ResolveStorageConnection(ctx context.Context, name string) (StorageAdapter, error)

    	// ResolveStorageConnectionName は実行コンテキストに基づいてデータストレージ接続の名前を解決します。
    	// jobExecution と stepExecution はモデルパッケージとの循環依存を避けるため interface{} として渡されます。
    	ResolveStorageConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error)
    }
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。

#### 2.3. GCSアダプターと汎用アイテムコンポーネントの連携

*   **目的**: 本タスクで実装するGCSアダプターが、どのように汎用的な`ItemReader`/`ItemWriter`によって利用され、ストレージ実装に依存しない処理を実現するかを明確にします。
*   **説明**:
    本タスクで実装する `gcsAdapter` は、`pkg/batch/adapter/storage/interfaces.go` で定義された汎用的な `StorageAdapter` インターフェースを実装します。この設計により、バッチフレームワークの `ItemReader` や `ItemWriter` といったコンポーネントは、GCSに直接依存することなく、この `StorageAdapter` インターフェースを介してファイル操作を行います。
    したがって、`ItemReader` や `ItemWriter` は、GCSアダプターが注入されればGCSから/へデータを処理し、将来的に実装されるであろうローカルファイルシステムアダプターが注入されればローカルファイルから/へデータを処理するといった、ストレージ実装に依存しない汎用的な振る舞いを実現します。これにより、GCSに特化した `step/reader` や `step/writer` が作成されることはありません。
    これらのコアコンポーネントは、`StorageAdapter` インターフェースのメソッド（`Upload`, `Download`, `ListObjects`, `DeleteObject`）のみを利用するため、GCSアダプターの実装が完了した後も、変更は不要です。
*   **テスト方法**:
    *   このセクションは説明のためであり、直接的なコード変更やテストは伴いません。

#### 1.2. `pkg/batch/adapter/storage/config/config.go` の作成

*   **目的**: ストレージ接続の設定を保持する構造体 (`StorageConfig`) を作成します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/config/config.go` (新規作成)
*   **変更内容**:
    ```go
    package config

    // StorageConfig holds configuration for a single storage connection.
    type StorageConfig struct {
    	Type            string `yaml:"type"`             // Type of storage (e.g., "gcs", "s3", "local", "ftp", "sftp").
    	BucketName      string `yaml:"bucket_name"`      // Default bucket name for operations.
    	CredentialsFile string `yaml:"credentials_file"` // Path to credentials file (e.g., service account key for GCS).
    	BaseDir         string `yaml:"base_dir"`         // Base directory for local file system operations.
    }

    // DatasourcesConfig holds a map of named storage configurations.
    type DatasourcesConfig map[string]StorageConfig
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。

### タスク 2: GCSアダプターの実装

#### 2.1. `pkg/batch/adapter/storage/gcs/adapter.go` の作成

*   **目的**: `StorageAdapter`, `StorageProvider`, `StorageConnectionResolver` インターフェースのGCS固有の実装を提供します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/gcs/adapter.go` (新規作成)
*   **変更内容**:
    ```go
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

        coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
        storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
        storageConfig "github.com/tigerroll/surfin/pkg/batch/adapter/storage/config"
        "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
    )

    // gcsAdapter は storage.StorageAdapter インターフェースを実装し、GCSへの具体的な操作を提供します。
    type gcsAdapter struct {
        client *storage.Client
        cfg    storageConfig.StorageConfig
        name   string
    }

    // Verify that gcsAdapter implements the storage.StorageAdapter interface.
    var _ storageAdapter.StorageAdapter = (*gcsAdapter)(nil)

    // NewGCSAdapter は新しい gcsAdapter インスタンスを作成します。
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

    // Close は内部のGCSクライアントをクローズし、関連するリソースを解放します。
    func (a *gcsAdapter) Close() error {
        if a.client != nil {
            return a.client.Close()
        }
        return nil
    }

    // Type はリソースのタイプ（"gcs"）を返します。
    func (a *gcsAdapter) Type() string {
        return a.cfg.Type
    }

    // Name はこの接続の名前を返します。
    func (a *gcsAdapter) Name() string {
        return a.name
    }

    // Upload は指定されたバケットとオブジェクト名にデータをアップロードします。
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
            wc.CloseWithError(err) // エラー発生時はWriterをエラー付きでクローズ
            return fmt.Errorf("failed to upload object '%s' to bucket '%s': %w", objectName, bucket, err)
        }

        if err := wc.Close(); err != nil {
            return fmt.Errorf("failed to close writer for object '%s' in bucket '%s': %w", objectName, bucket, err)
        }
        logger.Debugf("Uploaded object '%s' to bucket '%s'.", objectName, bucket)
        return nil
    }

    // Download は指定されたバケットとオブジェクト名からデータをダウンロードします。
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

    // ListObjects は指定されたバケットとプレフィックス内のオブジェクトをリストし、コールバック関数を呼び出します。
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

    // DeleteObject は指定されたバケットとオブジェクト名を削除します。
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

    // Config はこのアダプターが使用している設定を返します。
    func (a *gcsAdapter) Config() storageConfig.StorageConfig {
        return a.cfg
    }

    // GCSProvider は storage.StorageProvider インターフェースを実装し、GCS接続のライフサイクル管理を行います。
    type GCSProvider struct {
        cfg         *coreConfig.Config
        connections map[string]storageAdapter.StorageAdapter
        mu          sync.RWMutex
    }

    // Verify that GCSProvider implements the storage.StorageProvider interface.
    var _ storageAdapter.StorageProvider = (*GCSProvider)(nil)

    // NewGCSProvider は新しい GCSProvider インスタンスを作成します。
    func NewGCSProvider(cfg *coreConfig.Config) storageAdapter.StorageProvider {
        return &GCSProvider{
            cfg:         cfg,
            connections: make(map[string]storageAdapter.StorageAdapter),
        }
    }

    // GetConnection は指定された名前の StorageAdapter 接続を取得します。
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

    // CloseAll はこのプロバイダが管理する全ての接続をクローズします。
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
        p.connections = make(map[string]storageAdapter.StorageAdapter) // Clear connections
        if len(errs) > 0 {
            return fmt.Errorf("errors occurred while closing GCS connections: %v", errs)
        }
        return nil
    }

    // Type はこのプロバイダが扱うリソースのタイプ（"gcs"）を返します。
    func (p *GCSProvider) Type() string {
        return "gcs"
    }

    // ForceReconnect は指定された名前の既存の接続を強制的に閉じ、再確立します。
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

    // GCSConnectionResolver は storage.StorageConnectionResolver インターフェースを実装します。
    type GCSConnectionResolver struct {
        providers map[string]storageAdapter.StorageProvider
        cfg       *coreConfig.Config
    }

    // Verify that GCSConnectionResolver implements the storage.StorageConnectionResolver interface.
    var _ storageAdapter.StorageConnectionResolver = (*GCSConnectionResolver)(nil)

    // NewGCSConnectionResolver は新しい GCSConnectionResolver インスタンスを作成します。
    func NewGCSConnectionResolver(providers []storageAdapter.StorageProvider, cfg *coreConfig.Config) storageAdapter.StorageConnectionResolver {
        providerMap := make(map[string]storageAdapter.StorageProvider)
        for _, p := range providers {
            providerMap[p.Type()] = p
        }
        return &GCSConnectionResolver{
            providers: providerMap,
            cfg:       cfg,
        }
    }

    // ResolveConnection は指定された名前の StorageAdapter 接続インスタンスを解決します。
    func (r *GCSConnectionResolver) ResolveConnection(ctx context.Context, name string) (storageAdapter.StorageAdapter, error) {
        return r.ResolveStorageConnection(ctx, name)
    }

    // ResolveConnectionName は実行コンテキストに基づいてストレージ接続の名前を解決します。
    func (r *GCSConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
        // 現時点では、jobExecutionやstepExecutionからの動的な解決はサポートせず、defaultNameをそのまま返します。
        // 必要に応じて、ExecutionContextから接続名を解決するロジックを追加できます。
        return defaultName, nil
    }

    // ResolveStorageConnection は指定された名前の StorageAdapter 接続インスタンスを解決します。
    func (r *GCSConnectionResolver) ResolveStorageConnection(ctx context.Context, name string) (storageAdapter.StorageAdapter, error) {
        rawAdapterConfig, ok := r.cfg.Surfin.AdapterConfigs.(map[string]interface{})
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

        var tempCfg struct {
            Type string `yaml:"type"`
        }
        if err := mapstructure.Decode(namedConfig, &tempCfg); err != nil {
            return nil, fmt.Errorf("failed to decode storage type for '%s': %w", name, err)
        }

        provider, ok := r.providers[tempCfg.Type]
        if !ok {
            return nil, fmt.Errorf("no storage provider registered for type '%s'", tempCfg.Type)
        }

        return provider.GetConnection(name)
    }

    // ResolveStorageConnectionName は実行コンテキストに基づいてデータストレージ接続の名前を解決します。
    func (r *GCSConnectionResolver) ResolveStorageConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
        // 現時点では、jobExecutionやstepExecutionからの動的な解決はサポートせず、defaultNameをそのまま返します。
        // 必要に応じて、ExecutionContextから接続名を解決するロジックを追加できます。
        return defaultName, nil
    }
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。
    *   `go mod tidy` を実行し、必要な依存関係（`cloud.google.com/go/storage`, `github.com/mitchellh/mapstructure`, `google.golang.org/api/option`,
`google.golang.org/api/iterator`）が追加されることを確認します。

#### 2.2. `pkg/batch/adapter/storage/gcs/module.go` の作成

*   **目的**: FxアプリケーションにGCSアダプターのコンポーネントを提供するFxモジュールを定義します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/gcs/module.go` (新規作成)
*   **変更内容**:
    ```go
    package gcs

    import (
        "go.uber.org/fx"

        coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
        storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
    )

    // Module は GCS ストレージアダプターの Fx モジュールです。
    var Module = fx.Options(
        fx.Provide(NewGCSProvider),
        fx.Provide(func(providers []storageAdapter.StorageProvider, cfg *coreConfig.Config) storageAdapter.StorageConnectionResolver {
            return NewGCSConnectionResolver(providers, cfg)
        }),
        fx.Provide(func(p *GCSProvider) storageAdapter.StorageProvider {
            return p // GCSProviderをStorageProviderとして提供
        }),
    )
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。

### タスク 3: `weather` アプリケーションへの統合

#### 3.1. `example/weather/cmd/weather/main.go` の変更

*   **目的**: `weather` アプリケーションのFxグラフにGCSモジュールを追加し、アダプタープロバイダーの変数名を汎用化します。
*   **変更ファイル**:
    *   `example/weather/cmd/weather/main.go`
*   **変更内容**:
    `main` 関数内で、`adapterProviderOptions` スライスに `gcs.Module` を追加し、変数名を `dbProviderOptions` から `adapterProviderOptions` に変更します。
    ```go
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
        "github.com/tigerroll/surfin/pkg/batch/adapter/storage/gcs" // GCSモジュールをインポート
        "github.com/tigerroll/surfin/pkg/batch/core/config"
        "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
        "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

        "go.uber.org/fx"
        "gopkg.in/yaml.v3" // yaml package
    )

    // ... (省略) ...

    // Define Fx options for database providers. All GORM-based providers are included here.
    // [ここから変更]
    adapterProviderOptions := []fx.Option{ // 変数名を変更
        // gormmodule.Module is removed as NewGormTransactionManagerFactory is already provided in internal/app/module.go.
        mysql.Module,
        postgres.Module,
        sqlite.Module, // SQLite module
        gcs.Module,    // GCSストレージアダプターモジュールを追加
    }

    // Run the application.
    // Cast embeddedConfig and embeddedJSL to their respective type aliases and add jobDoneChan.
    app.RunApplication(ctx, envFilePath, config.EmbeddedConfig(embeddedConfig), jsl.JSLDefinitionBytes(embeddedJSL), applicationMigrationsFS, adapterProviderOptions, jobDoneChan)
    // [ここまで変更]
    // Exit the process with exit code 0 after application execution completes.
    os.Exit(0)
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。

## 変更不要なファイル

以下のファイルは、今回のGCSアダプターの実装において変更は不要です。

*   `pkg/batch/core/config/config.go`
*   `pkg/batch/core/config/loader.go`
*   `pkg/batch/core/adapter/interfaces.go`
*   `pkg/batch/core/config/bootstrap/module.go`
*   `example/hello-world/cmd/hello-world/main.go` (このタスクでは変更しません)
