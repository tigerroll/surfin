# ParquetWriter 設計

## 1. 目的

`ParquetWriter` は、Surfin バッチフレームワークにおいて、構造化されたデータを Apache Parquet 形式で効率的にストレージに書き込むための汎用的な `port.ItemWriter` 実装です。このコンポーネントは、以下の目的を達成します。

*   **Parquet 書き込みロジックの抽象化**: Parquet ファイルの生成、データバッファリング、パーティショニング、ストレージへのアップロードといった複雑なロジックをカプセル化し、再利用可能なコンポーネントとして提供します。
*   **JSL (Job Specification Language) からの設定**: JSL を通じて設定可能とすることで、バッチジョブ定義の柔軟性と宣言性を高めます。
*   **効率的なデータ処理**: `port.ItemWriter` インターフェースを通じてチャンク単位でデータを受け取り、内部でバッファリングした後、`Close` 時にまとめて Parquet ファイルとして書き出すことで、I/O 効率を向上させます。
*   **Hive スタイルのパーティショニングサポート**: ユーザーが指定する関数に基づいて、データを日付などのキーでパーティション分割し、Hive スタイルのディレクトリ構造で出力する機能を提供します。

## 2. 構造体定義

### 2.1. `ParquetWriterConfig`

`ParquetWriter` の設定を保持する構造体です。

```go
type ParquetWriterConfig struct {
	StorageRef      string `mapstructure:"storageRef"`      // 使用するストレージ接続の名前 (例: "gcs_bucket", "s3_bucket")
	OutputBaseDir   string `mapstructure:"outputBaseDir"`   // エクスポートされるファイルのストレージバケット内のベースディレクトリ (例: "weather/hourly_forecast")
	CompressionType string `mapstructure:"compressionType"` // Parquet ファイルの圧縮タイプ (例: "SNAPPY", "GZIP", "NONE")
}
```

### 2.2. `ParquetWriter`

`port.ItemWriter` インターフェースを実装する主要な構造体です。

```go
type ParquetWriter[T any] struct {
	name                      string
	config                    *ParquetWriterConfig
	storageConnectionResolver storage.StorageConnectionResolver
	itemPrototype             T // Parquet スキーマのリフレクションに使用するアイテム型のプロトタイプインスタンス。
	partitionKeyFunc          func(T) (string, error) // アイテムからパーティションキー (例: "YYYY-MM-DD") を抽出する関数。

	// 内部書き込み状態
	storageConn          storage.StorageConnection // 解決されたストレージ接続インスタンス
	bufferedItems        map[string][]T          // キー: パーティションキー (例: "dt=YYYY-MM-DD")、値: そのパーティションのバッファされたアイテム。
	totalRecordsBuffered int64                   // バッファリングされた全レコードの合計数。
	stepExecutionContext model.ExecutionContext  // フレームワークによって管理される実行コンテキスト。
}
```

## 3. コンストラクタ

### 3.1. `NewParquetWriter`

`ParquetWriter` の新しいインスタンスを作成します。

```go
func NewParquetWriter[T any](
	name string,
	properties map[string]string,
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype T,
	partitionKeyFunc func(T) (string, error),
) (port.ItemWriter[T], error) {
	// properties を ParquetWriterConfig にデコード
	// ParquetWriter インスタンスを初期化して返す
}
```

*   `name string`: Writer の一意な名前。ロギングや識別に使用されます。
*   `properties map[string]string`: JSL から渡される設定プロパティ。`ParquetWriterConfig` にデコードされます。
*   `storageConnectionResolver storage.StorageConnectionResolver`: ストレージ接続を解決するためのインターフェース。
*   `itemPrototype T`: Parquet スキーマを推論するために使用される、書き込むアイテムの型のゼロ値またはインスタンス。`xintongsys/parquet-go` ライブラリが Go の構造体タグ (`parquet:"..."`) を利用してスキーマを生成するために必要です。
*   `partitionKeyFunc func(T) (string, error)`: 各アイテムからパーティションキー（例: 日付文字列 "YYYY-MM-DD"）を抽出するための関数。このキーに基づいてデータがパーティション分割されます。

### 3.2. `NewParquetWriterBuilder`

JSL の `ComponentBuilder` シグネチャに適合する関数を生成します。これにより、Fx (Dependency Injection) を介して `ParquetWriter` を JSL コンポーネントとして登録できます。

```go
func NewParquetWriterBuilder[T any](
	storageConnectionResolver storage.StorageConnectionResolver,
	itemPrototype T,
	partitionKeyFunc func(T) (string, error),
) func(properties map[string]string) (port.ItemWriter[T], error) {
	// NewParquetWriter を呼び出すクロージャを返す
}
```

*   このビルダー関数は、特定の `itemPrototype` と `partitionKeyFunc` を受け取り、それらを内部に保持した `NewParquetWriter` を呼び出す関数を返します。これにより、JSL からは汎用的な `ParquetWriter` を呼び出しつつ、Go の型システムと依存性注入の恩恵を受けられます。

## 4. ライフサイクルメソッドの実装

`ParquetWriter` は `port.ItemWriter` インターフェースを実装します。

### 4.1. `Open(ctx context.Context, ec model.ExecutionContext) error`

*   **役割**: Writer の初期化とリソースの準備。
*   **動作**:
    1.  `storageConnectionResolver` を使用して、設定された `StorageRef` に基づいてストレージ接続を解決します。
    2.  解決された `storage.StorageConnection` インスタンスを内部に保持します。
    3.  `stepExecutionContext` を保存します。ParquetWriter は再起動可能な状態を自身で管理しないため、このコンテキストは主にフレームワークによる利用のために保持されます。

### 4.2. `Write(ctx context.Context, items []T) error`

*   **役割**: 受け取ったアイテムのチャンクを内部バッファに蓄積します。
*   **動作**:
    1.  受け取った `items` スライス内の各アイテムをループ処理します。
    2.  `partitionKeyFunc` を使用して、各アイテムのパーティションキーを抽出します。
    3.  抽出したパーティションキー（例: "dt=YYYY-MM-DD" 形式）に基づいて、アイテムを内部マップ `bufferedItems` に追加します。
    4.  `totalRecordsBuffered` カウンターを更新します。
    5.  このメソッドでは、実際の Parquet ファイルへの書き込みやストレージへのアップロードは行いません。

### 4.3. `Close(ctx context.Context) error`

*   **役割**: バッファリングされたすべてのデータを Parquet ファイルとして確定し、ストレージにアップロードします。
*   **動作**:
    1.  `bufferedItems` マップをループし、パーティションキーごとに処理を行います。
    2.  各パーティションキーに対して、新しい `bytes.Buffer` を作成します。
    3.  `xintongsys/parquet-go` ライブラリの `writer.NewParquetWriterFromWriter` を使用して、そのバッファに書き込む Parquet ライターを作成します。この際、`itemPrototype` を使用して Parquet スキーマを推論します。
    4.  設定された `CompressionType` を Parquet ライターに適用します。
    5.  そのパーティションに属するすべてのバッファリングされたアイテムを Parquet ライターに書き込みます。
    6.  `pw.WriteStop()` を呼び出して Parquet ファイルを確定します。この際、ライブラリのパニックを捕捉し、エラーに変換するための `defer recover()` ロジックを含めます。
    7.  Hive スタイルのパス (`OutputBaseDir/dt=YYYY-MM-DD/`) と一意なファイル名（例: `data_YYYYMMDD_HHMMSS.parquet`）を生成します。
    8.  `storageConn.Upload` を使用して、バッファの内容を生成されたパスにアップロードします。
    9.  すべてのパーティションの処理が完了したら、内部バッファ (`bufferedItems`) をクリアし、`totalRecordsBuffered` をリセットします。
    10. 最後に、解決されたストレージ接続をクローズします。

### 4.4. `SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error`

*   **役割**: Writer の実行コンテキストを設定します。
*   **動作**: 提供された `model.ExecutionContext` を内部の `stepExecutionContext` フィールドに保存します。`ParquetWriter` は再起動可能な状態を自身で管理しないため、このメソッドは主にフレームワークによる利用のために存在します。

### 4.5. `GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)`

*   **役割**: Writer の現在の実行コンテキストを取得します。
*   **動作**: 内部に保存されている `stepExecutionContext` を返します。

## 5. その他のメソッド

### 5.1. `GetTargetResourceName() string`

*   **役割**: この Writer が書き込むターゲットリソースの名前を返します。
*   **動作**: 設定された `StorageRef` の値を返します。

### 5.2. `GetResourcePath() string`

*   **役割**: この Writer が書き込むターゲットリソース内のパスまたは識別子を返します。
*   **動作**: 設定された `OutputBaseDir` の値を返します。

## 6. 設計上の考慮事項

*   **ジェネリクス `[T any]`**: 任意の Go 構造体を Parquet に書き込めるように、ジェネリクスを使用します。これにより、特定のデータ型に依存しない汎用的な Writer となります。
*   **`itemPrototype` の利用**: `xintongsys/parquet-go` ライブラリは、Go 構造体のタグ (`parquet:"..."`) を利用して Parquet スキーマを推論します。`itemPrototype` をコンストラクタで受け取ることで、このスキーマ推論を可能にします。
*   **パーティショニングの柔軟性**: `partitionKeyFunc` を外部から注入可能とすることで、日付、カテゴリ、ユーザーIDなど、様々な基準でのパーティショニングロジックに対応できます。
*   **エラーハンドリング**: 特に `xintongsys/parquet-go` ライブラリの `WriteStop()` メソッドがパニックを起こす可能性があるため、`defer recover()` を用いた堅牢なエラー処理を組み込みます。
*   **メモリ使用量**: `Write` メソッドはデータを内部バッファに蓄積するため、`Close` が呼び出されるまでの間に大量のデータがバッファリングされる可能性があります。非常に大きなデータセットを扱う場合は、メモリ使用量に注意が必要です。必要に応じて、バッファサイズの上限を設けるなどの対策を検討できます（ただし、この設計では明示的な上限は設けていません）。
*   **トランザクション**: `ParquetWriter` は `port.ItemWriter` であり、フレームワークのトランザクション管理（ChunkStep など）に参加します。`Write` メソッドはトランザクションのコンテキスト内で呼び出され、`Close` メソッドはチャンク処理の最後に呼び出されることを想定しています。
