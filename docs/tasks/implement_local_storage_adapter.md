# タスクチケット: ローカルストレージアダプターの実装

## 概要

このタスクは、バッチフレームワークにローカルファイルシステムアダプターを実装し、ローカルファイルシステムへのデータアクセスを可能にすることを目的とします。`docs/strategy/storage_adapter_design.md` に定義された設計に基づき、共通インターフェースの活用、ローカルファイルシステム固有の実装、およびアプリケーションへの統合を行います。

**本タスクで実装されるローカルストレージアダプターは、汎用的な `StorageAdapter` インターフェースを実装するため、バッチフレームワークのアイテムリーダーやアイテムライターといったコアコンポーネントは、ローカルファイルシステムに特化することなく、この抽象化されたインターフェースを通じてファイルシステムと連携できます。これにより、GCSやS3などの他のクラウドストレージ向けのアダプターが追加された場合でも、同じリーダー/ライターのロジックを再利用することが可能になります。**

## タスクリスト

### タスク 1: 設定構造体の更新

#### 1.1. `pkg/batch/adapter/storage/config/config.go` の更新

*   **目的**: ローカルファイルシステムアダプターが使用するベースディレクトリを指定するための `BaseDir` フィールドを `StorageConfig` 構造体に追加します。このフィールドは `docs/strategy/storage_adapter_design.md` で既に定義されていますが、実装タスクとして改めて確認します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/config/config.go`
*   **変更内容**:
    `StorageConfig` 構造体に `BaseDir` フィールドを追加します。

    ```go
    // pkg/batch/adapter/storage/config/config.go
    package config

    // StorageConfig holds configuration for a single storage connection.
    type StorageConfig struct {
        Type            string `yaml:"type"`             // Type of storage (e.g., "gcs", "s3", "local", "ftp", "sftp").
        BucketName      string `yaml:"bucket_name"`      // Default bucket name for operations. (GCS/S3向け)
        CredentialsFile string `yaml:"credentials_file"` // Path to credentials file (e.g., service account key for GCS). (GCS/S3向け)
        BaseDir         string `yaml:"base_dir"`         // Base directory for local file system operations. (Local向け)
    }

    // DatasourcesConfig holds a map of named storage configurations.
    type DatasourcesConfig map[string]StorageConfig
    ```
*   **テスト方法**:
    *   Goのビルド (`go build ./...`) を実行し、構文エラーがないことを確認します。

### タスク 2: ローカルストレージアダプターの実装

#### 2.1. `pkg/batch/adapter/storage/local/adapter.go` の作成

*   **目的**: `StorageAdapter`, `StorageProvider`, `StorageConnectionResolver` インターフェースのローカルファイルシステム固有の実装を提供します。
*   **変更ファイル**:
    *   `pkg/batch/adapter/storage/local/adapter.go` (新規作成)
*   **変更内容**:
    以下の内容でファイルを新規作成します。

    ```go
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
        "time"

        "github.com/mitchellh/mapstructure"

        coreConfig "github.com/tigerroll/surfin/pkg/batch/core/config"
        storageAdapter "github.com/tigerroll/surfin/pkg/batch/adapter/storage"
        storageConfig "github.com/tigerroll/surfin/pkg/batch/adapter/storage/config"
        "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
    )

    // localAdapter は storage.StorageAdapter インターフェースを実装し、ローカルファイルシステムへの具体的な操作を提供します。
    type localAdapter struct {
        cfg  storageConfig.StorageConfig
        name string
    }

    // Verify that localAdapter implements the storage.StorageAdapter interface.
    var _ storageAdapter.StorageAdapter = (*localAdapter)(nil)

    // NewLocalAdapter は新しい localAdapter インスタンスを作成します。
    func NewLocalAdapter(cfg storageConfig.StorageConfig, name string) (storageAdapter.StorageAdapter, error) {
        if cfg.BaseDir == "" {
            return nil, fmt.Errorf("BaseDir must be specified for local storage adapter '%s'", name)
        }
        // BaseDirが存在し、ディレクトリであることを確認
        info, err := os.Stat(cfg.BaseDir)
        if os.IsNotExist(err) {
            // ディレクトリが存在しない場合は作成を試みる
            if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
                return nil, fmt.Errorf("failed to create BaseDir '%s' for local storage adapter '%s': %w", cfg.BaseDir, name, err)
            }
            logger.Infof("Created BaseDir '%s' for local storage adapter '%s'.", cfg.BaseDir, name)
        } else if err != nil {
            return nil, fmt.Errorf("failed to stat BaseDir '%s' for local storage adapter '%s': %w", cfg.BaseDir, name, err)
        } else if !info.IsDir() {
            return nil, fmt.Errorf("BaseDir '%s' for local storage adapter '%s' is not a directory", cfg.BaseDir, name)
        }

        return &localAdapter{
            cfg:  cfg,
            name: name,
        }, nil
    }

    // Close はローカルファイルシステムアダプターは特別なリソースを保持しないため、何もしません。
    func (a *localAdapter) Close() error {
        logger.Debugf("Local storage connection '%s' closed (no-op).", a.name)
        return nil
    }

    // Type はリソースのタイプ（"local"）を返します。
    func (a *localAdapter) Type() string {
        return "local"
    }

    // Name はこの接続の名前を返します。
    func (a *localAdapter) Name() string {
        return a.name
    }

    // resolvePath は BaseDir と bucket/objectName を結合して絶対パスを返します。
    // bucket はローカルファイルシステムではサブディレクトリとして扱われます。
    func (a *localAdapter) resolvePath(bucket, objectName string) (string, error) {
        if bucket == "" {
            bucket = a.cfg.BucketName // 設定のBucketNameをデフォルトとして使用
        }
        if bucket == "" {
            // bucketが指定されていない場合、BaseDir直下にobjectNameを配置
            return filepath.Join(a.cfg.BaseDir, objectName), nil
        }
        // bucketが指定されている場合、BaseDir/bucket/objectName
        fullPath := filepath.Join(a.cfg.BaseDir, bucket, objectName)
        // パスがBaseDirの外に出ないように検証
        if !strings.HasPrefix(fullPath, a.cfg.BaseDir) {
            return "", fmt.Errorf("resolved path '%s' is outside of BaseDir '%s'", fullPath, a.cfg.BaseDir)
        }
        return fullPath, nil
    }

    // Upload は指定されたバケット（ディレクトリ）とオブジェクト名（ファイルパス）にデータを書き込みます。
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

        if _, err := io.Copy(file, data); err != nil {
            return fmt.Errorf("failed to write data to file '%s': %w", fullPath, err)
        }
        logger.Debugf("Uploaded object '%s' to local path '%s'.", objectName, fullPath)
        return nil
    }

    // Download は指定されたバケット（ディレクトリ）とオブジェクト名（ファイルパス）からデータを読み込みます。
    func (a *localAdapter) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
        fullPath, err := a.resolvePath(bucket, objectName)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve path for download: %w", err)
        }

        file, err := os.Open(fullPath)
        if err != nil {
            return nil, fmt.Errorf("failed to open file '%s': %w", fullPath, err)
        }
        logger.Debugf("Downloaded object '%s' from local path '%s'.", objectName, fullPath)
        return file, nil
    }

    // ListObjects は指定されたバケット（ディレクトリ）とプレフィックス内のファイルをリストし、コールバック関数を呼び出します。
    func (a *localAdapter) ListObjects(ctx context.Context, bucket, prefix string, fn func(objectName string) error) error {
        basePath, err := a.resolvePath(bucket, "") // bucketをディレクトリとして扱うためobjectNameは空
        if err != nil {
            return fmt.Errorf("failed to resolve base path for list: %w", err)
        }

        // prefixが指定されている場合、basePathに結合して走査開始ディレクトリとする
        startPath := filepath.Join(basePath, prefix)

        err = filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                // ファイルが見つからないなどのエラーはスキップ
                if os.IsNotExist(err) {
                    return nil
                }
                return fmt.Errorf("error walking path '%s': %w", path, err)
            }
            if d.IsDir() {
                return nil // ディレクトリはスキップ
            }

            // BaseDirからの相対パスを取得し、objectNameとして渡す
            relPath, err := filepath.Rel(basePath, path)
            if err != nil {
                return fmt.Errorf("failed to get relative path for '%s' from '%s': %w", path, basePath, err)
            }

            // prefixに一致するか確認
            if !strings.HasPrefix(relPath, prefix) {
                return nil
            }

            if err := fn(relPath); err != nil {
                return fmt.Errorf("callback function failed for object '%s': %w", relPath, err)
            }
            return nil
        })

        if err != nil {
            return fmt.Errorf("failed to list objects in '%s' with prefix '%s': %w", basePath, prefix, err)
        }
        logger.Debugf("Listed objects in local path '%s' with prefix '%s'.", basePath, prefix)
        return nil
    }

    // DeleteObject は指定されたバケット（ディレクトリ）とオブジェクト名（ファイルパス）を削除します。
    func (a *localAdapter) DeleteObject(ctx context.Context, bucket, objectName string) error {
        fullPath, err := a.resolvePath(bucket, objectName)
        if err != nil {
            return fmt.Errorf("failed to resolve path for delete: %w", err)
        }

        if err := os.Remove(fullPath); err != nil {
            if os.IsNotExist(err) {
                logger.Debugf("Attempted to delete non-existent object '%s' from local path '%s'. Skipping.", objectName, fullPath)
                return nil // 存在しない場合はエラーとしない
            }
            return fmt.Errorf("failed to delete file '%s': %w", fullPath, err)
        }
        logger.Debugf("Deleted object '%s' from local path '%s'.", objectName, fullPath)
        return nil
    }

    // Config はこのアダプターが使用している設定を返します。
    func (a *localAdapter) Config() storageConfig.StorageConfig {
        return a.cfg
    }

    // LocalProvider は storage.StorageProvider インターフェースを実装し、ローカルファイルシステム接続のライフサイクル管理を行います。
    type LocalProvider struct {
        cfg         *coreConfig.Config
        connections map[string]storageAdapter.StorageAdapter
        mu          sync.RWMutex
    }

    // Verify that LocalProvider implements the storage.StorageProvider interface.
    var _ storageAdapter.StorageProvider = (*LocalProvider)(nil)

    // NewLocalProvider は新しい LocalProvider インスタンスを作成します。
    func NewLocalProvider(cfg *coreConfig.Config) storageAdapter.StorageProvider {
        return &LocalProvider{
            cfg:         cfg,
            connections: make(map[string]storageAdapter.StorageAdapter),
        }
    }

    // GetConnection は指定された名前の StorageAdapter 接続を取得します。
    func (p *LocalProvider) GetConnection(name string) (storageAdapter.StorageAdapter, error) {
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

        conn, err = NewLocalAdapter(storageCfg, name)
        if err != nil {
            return nil, fmt.Errorf("failed to create local adapter for '%s': %w", name, err)
        }

        p.connections[name] = conn
        logger.Infof("Created new local storage connection '%s'.", name)
        return conn, nil
    }

    // CloseAll はこのプロバイダが管理する全ての接続をクローズします。
    func (p *LocalProvider) CloseAll() error {
        p.mu.Lock()
        defer p.mu.Unlock()

        var errs []error
        for name, conn := range p.connections {
            if err := conn.Close(); err != nil {
                errs = append(errs, fmt.Errorf("failed to close local storage connection '%s': %w", name, err))
            } else {
                logger.Infof("Closed local storage connection '%s'.", name)
            }
        }
        p.connections = make(map[string]storageAdapter.StorageAdapter) // Clear connections
        if len(errs) > 0 {
            return fmt.Errorf("errors occurred while closing local storage connections: %v", errs)
        }
        return nil
    }

    // Type はこのプロバイダが扱うリソースのタイプ（"local"）を返します。
    func (p *LocalProvider) Type() string {
        return "local"
    }

    // ForceReconnect は指定された名前の既存の接続を強制的に閉じ、再確立します。
    func (p *LocalProvider) ForceReconnect(name string) (storageAdapter.StorageAdapter, error) {
        p.mu.Lock()
        defer p.mu.Unlock()

        if conn, ok := p.connections[name]; ok {
            if err := conn.Close(); err != nil {
                logger.Warnf("Failed to gracefully close local storage connection '%s' during force reconnect: %v", name, err)
            }
            delete(p.connections, name)
            logger.Infof("Closed existing local storage connection '%s' for force reconnect.", name)
        }

        // Re-create the connection
        return p.GetConnection(name)
    }

    // LocalConnectionResolver は storage.StorageConnectionResolver インターフェースを実装します。
    type LocalConnectionResolver struct {
        providers map[string]storageAdapter.StorageProvider
        cfg       *coreConfig.Config
    }

    // Verify that LocalConnectionResolver implements the storage.StorageConnectionResolver interface.
    var _ storageAdapter.StorageConnectionResolver = (*LocalConnectionResolver)(nil)

    // NewLocalConnectionResolver は新しい LocalConnectionResolver インスタンスを作成します。
    func NewLocalConnectionResolver(providers []storageAdapter.StorageProvider, cfg *coreConfig.Config) storageAdapter.StorageConnectionResolver {
        providerMap := make(map[string]storageAdapter.StorageProvider)
        for _, p := range providers {
            providerMap[p.Type()] = p
        }
        return &LocalConnectionResolver{
            providers: providerMap,
            cfg:       cfg,
        }
    }

    // ResolveConnection は指定された名前の StorageAdapter 接続インスタンスを解決します。
    func (r *LocalConnectionResolver) ResolveConnection(ctx context.Context, name string) (storageAdapter.StorageAdapter, error) {
        return r.ResolveStorageConnection(ctx, name)
    }

    // ResolveConnectionName は実行コンテキストに基づいてストレージ接続の名前を解決します。
    func (r *LocalConnectionResolver) ResolveConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
        // 現時点では、jobExecutionやstepExecutionからの動的な解決はサポートせず、defaultNameをそのまま返します。
        // 必要に応じて、ExecutionContextから接続名を解決するロジックを追加できます。
        return defaultName, nil
    }

    // ResolveStorageConnection は指定された名前の StorageAdapter 接続インスタンスを解決します。
    func (r *LocalConnectionResolver) ResolveStorageConnection(ctx context.Context, name string) (storageAdapter.StorageAdapter, error) {
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
    func (r *LocalConnectionResolver) ResolveStorageConnectionName(ctx context.Context, jobExecution interface{}, stepExecution interface{}, defaultName string) (string, error) {
        // 現時点では、jobExecutionやstepExecutionからの動的な解決はサポートせず、defaultNameをそのまま返します。
        // 必要に応じて、ExecutionContextから接続名を解決するロジックを追加できます。
        return defaultName, nil
    }
    ```
