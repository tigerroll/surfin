# ParquetWriter 実装ステップ

このドキュメントは、`ParquetWriter` コンポーネントを実装するための段階的な手順を整理したものです。

## 1. 構造体の定義

*   `ParquetWriterConfig` 構造体を定義します。
    ```go
    type ParquetWriterConfig struct {
    	StorageRef      string `mapstructure:"storageRef"`
    	OutputBaseDir   string `mapstructure:"outputBaseDir"`
    	CompressionType string `mapstructure:"compressionType"`
    }
    ```
*   `ParquetWriter[T any]` 構造体を定義します。
    ```go
    type ParquetWriter[T any] struct {
    	name                      string
    	config                    *ParquetWriterConfig
    	storageConnectionResolver storage.StorageConnectionResolver
    	itemPrototype             T
    	partitionKeyFunc          func(T) (string, error)

    	storageConn          storage.StorageConnection
    	bufferedItems        map[string][]T
    	totalRecordsBuffered int64
    	stepExecutionContext model.ExecutionContext
    }
    ```

## 2. コンストラクタの定義

*   `NewParquetWriter` 関数を定義し、`ParquetWriter` の新しいインスタンスを初期化するロジックを実装します。
    ```go
    func NewParquetWriter[T any](
    	name string,
    	properties map[string]string,
    	storageConnectionResolver storage.StorageConnectionResolver,
    	itemPrototype T,
    	partitionKeyFunc func(T) (string, error),
    ) (port.ItemWriter[T], error) {
    	// 実装ロジック
    }
    ```
*   `NewParquetWriterBuilder` 関数を定義し、JSL の `ComponentBuilder` シグネチャに適合するクロージャを返します。
    ```go
    func NewParquetWriterBuilder[T any](
    	storageConnectionResolver storage.StorageConnectionResolver,
    	itemPrototype T,
    	partitionKeyFunc func(T) (string, error),
    ) func(properties map[string]string) (port.ItemWriter[T], error) {
    	// 実装ロジック
    }
    ```

## 3. `port.ItemWriter` インターフェースの実装

*   `Open(ctx context.Context, ec model.ExecutionContext) error` メソッドを実装します。
    *   ストレージ接続の解決と内部状態の初期化を行います。
*   `Write(ctx context.Context, items []T) error` メソッドを実装します。
    *   受け取ったアイテムをパーティションキーに基づいて内部バッファに蓄積するロジックを記述します。
*   `Close(ctx context.Context) error` メソッドを実装します。
    *   バッファリングされたデータをParquetファイルとして確定し、ストレージにアップロードするロジックを記述します。
    *   `xintongsys/parquet-go` ライブラリのパニックを捕捉する `defer recover()` ロジックを含めます。
*   `SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error` メソッドを実装します。
    *   実行コンテキストを保存します。
*   `GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)` メソッドを実装します。
    *   保存された実行コンテキストを返します。

## 4. その他のメソッドの実装

*   `GetTargetResourceName() string` メソッドを実装します。
    *   `StorageRef` の値を返します。
*   `GetResourcePath() string` メソッドを実装します。
    *   `OutputBaseDir` の値を返します。
