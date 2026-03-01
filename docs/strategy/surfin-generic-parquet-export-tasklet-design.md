# GenericParquetExportTasklet の実装

## 1. 目的

本ドキュメントは、`GenericParquetExportTasklet` は、Surfin バッチフレームワークにおいて、特定の Go 構造体型に依存せず、JSL (Job Specification Language) を通じて動的に設定可能な汎用的な Parquet
ファイル出力タスクレットを提供します。これにより、様々なデータソースから読み込んだデータを Apache Parquet 形式で効率的にストレージに書き出すことが可能です。

主な目的は以下の通りです。

*   **汎用性**: 特定のエンティティ型（例: `HourlyForecast`）に固定されず、任意の Go 構造体 `T` を Parquet に出力できます。
*   **JSL による動的設定**: データベースのテーブル名、SELECT 句、ORDER BY 句、Parquet のパーティションキー生成ロジック（対象カラムとフォーマット）を JSL から柔軟に設定可能です。
*   **型安全性の維持**: Go のジェネリクスと Fx (Dependency Injection) を活用し、実行時のリフレクションを最小限に抑えつつ、コンパイル時の型安全性を可能な限り維持します。
*   **再利用性**: データベースからの読み込み (`SqlCursorReader`) と Parquet への書き込み (`ParquetWriter`) のロジックをカプセル化し、再利用可能なコンポーネントとして提供します。

## 2. 設計思想

`GenericParquetExportTasklet` は、以下の設計思想に基づき実装されています。

*   **ジェネリクス `[T any]`**: 処理対象となるデータの Go 構造体型をジェネリクス `T` で抽象化することで、コードの汎用性を高めます。
*   **Fx を用いた型特化ロジックの注入**: JSL は文字列ベースの設定しかできないため、Go の具体的な型 `T` に依存するロジック（例: `sql.Rows` から `T` へ のスキャン関数、`T` から Parquet
スキーマを推論するためのプロトタイプインスタンス）は、Fx の DI コンテナを通じて注入します。これにより、タスクレット本体は汎用的なまま、特定の型に特化した振る舞いを実現します。
*   **JSL による動的 SQL 構築**: データベースからのデータ読み込みに必要な SQL クエリ（テーブル名、SELECT 句、ORDER BY 句）は、JSL のプロパティとして受け取り、実行時に動的に構築します。
*   **リフレクションによるパーティションキー生成**: Parquet のパーティションキーは、JSL で指定された Go 構造体のフィールド名とフォーマット文字列に基づいて、リフレクションを用いて動的に値を取得し、生成します。これにより、JSL 側でパーティションロジックを柔軟に定義できます。
*   **並行処理パイプライン**: データベースからの読み込みと Parquet ファイルへの書き込みは、goroutine とチャネルを用いた並行処理パイプラインとして実装し、I/O 効率を最大化します。

## 3. 構造体定義

### 3.1. `GenericParquetExportTaskletConfig`

JSL から受け取る設定を保持する構造体です。

```go
type GenericParquetExportTaskletConfig struct {
    DbRef                  string `mapstructure:"dbRef"`
    StorageRef             string `mapstructure:"storageRef"`
    OutputBaseDir          string `mapstructure:"outputBaseDir"`
    ReadBufferSize         int    `mapstructure:"readBufferSize"`
    ParquetCompressionType string `mapstructure:"parquetCompressionType"`
    TableName              string `mapstructure:"tableName"`
    SQLSelectColumns       string `mapstructure:"sqlSelectColumns"` // データベースから選択するカラムのSQL SELECT句
    SQLOrderBy             string `mapstructure:"sqlOrderBy"`       // データベースからデータを取得する際のORDER BY句
    // PartitionDefinitions defines multiple partition keys and their formats.
    // Each definition corresponds to a level in the hierarchical partition path.
    PartitionDefinitions []PartitionDefinition `mapstructure:"partitionDefinitions"`
}

// PartitionDefinition defines a single partition key within a multi-level partitioning scheme.
type PartitionDefinition struct {
    // Key is the name of the struct field to use for this partition.
    Key string `mapstructure:"key"`
    // Format is the format string for the partition key.
    // For time.Time fields, use Go's reference time layout (e.g., "2006-01-02", "15").
    // For other types, it can be a format specifier for fmt.Sprintf (e.g., "dt=%s", "code=%d").
    // If empty, the value will be converted to string using its default string representation.
    Format string `mapstructure:"format"`
    // Prefix is an optional prefix for the partition directory name (e.g., "dt=", "hour=").
    // If empty, the key name will be used as prefix.
    Prefix string `mapstructure:"prefix"`
}
```

### 3.2. GenericParquetExportTasklet[T]

port.Tasklet インターフェースを実装する主要な構造体です。

```go
type GenericParquetExportTasklet[T any] struct {
    config                    *GenericParquetExportTaskletConfig
    dbConnectionResolver      database.DBConnectionResolver
    storageConnectionResolver storage.StorageConnectionResolver
    parquetWriter             port.ItemWriter[T] // Parquetファイルへの書き込みを担当するItemWriter
    partitionKeyFunc          func(T) (string, error) // アイテムからパーティションキーを抽出する関数
}
```

## 4. コンストラクタ

### 4.1. NewGenericParquetExportTasklet

GenericParquetExportTasklet の新しいインスタンスを作成します。

```go
func NewGenericParquetExportTasklet[T any](
    properties map[string]interface{},
    dbConnectionResolver database.DBConnectionResolver,
    storageConnectionResolver storage.StorageConnectionResolver,
    itemPrototype *T, // Parquet スキーマ推論用プロトタイプ (Fx から注入される、型 T のゼロ値インスタンスへのポインタ)
) (port.Tasklet, error) {
    // 1. properties を GenericParquetExportTaskletConfig にデコード
    // 2. 必須設定のバリデーション
    // 3. リフレクションを用いて動的な partitionKeyFunc を生成 (PartitionDefinitions に基づく)
    // 4. writerComponent.NewParquetWriter を呼び出し、ParquetWriter を初期化
    // 5. GenericParquetExportTasklet インスタンスを初期化して返す
}
```

*   `itemPrototype *T`: `xintongsys/parquet-go` ライブラリが Parquet スキーマを推論するために使用する、書き込むアイテムの型のゼロ値またはインスタンスへのポインタ。Fx から注入されます。
*   `partitionKeyFunc func(T) (string, error)`: 各アイテムからパーティションキー（例: 日付文字列 "YYYY-MM-DD"）を抽出するための関数。この関数は NewGenericParquetExportTasklet 内部で、JSL の PartitionKeyColumn と PartitionKeyFormat
    に基づいて動的に生成されます。

### 4.2. NewGenericParquetExportTaskletBuilder

JSL の ComponentBuilder シグネチャに適合する関数を生成します。これにより、Fx (Dependency Injection) を介して GenericParquetExportTasklet を JSL コンポーネントとして登録できます。

```go
func NewGenericParquetExportTaskletBuilder[T any](
    dbConnectionResolver database.DBConnectionResolver, // Fx から注入
    storageConnectionResolver storage.StorageConnectionResolver, // Fx から注入
    itemPrototype *T, // Fx から注入
) jsl.ComponentBuilder {
    return func(
        cfg *config.Config,
        resolver port.ExpressionResolver,
        resourceProviders map[string]coreAdapter.ResourceProvider,
        properties map[string]interface{},
    ) (interface{}, error) {
        // NewGenericParquetExportTasklet を呼び出すクロージャを返す
        // Fx から注入された依存関係と JSL properties を NewGenericParquetExportTasklet に渡す
    }
}
```

*   このビルダー関数は、特定の `itemPrototype` を受け取り、それを内部に保持した `NewGenericParquetExportTasklet` を呼び出す関数を返します。これにより、JSL からは汎用的なタスクレットを呼び出しつつ、Go の型システムと依存性注入の恩恵を受けられます。

## 5. ライフサイクルメソッドの実装

GenericParquetExportTasklet は port.Tasklet インターフェースを実装します。

### 5.1. Open(ctx context.Context, ec model.ExecutionContext) error

*   **役割**: タスクレットの初期化とリソースの準備。
*   **動作**:
    1.  内部で利用する ParquetWriter の Open メソッドを呼び出します。
    2.  stepExecutionContext を保存します。

### 5.2. Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error)

*   **役割**: タスクレットのコアロジックを実行します。
*   **動作**:
    1.  dbConnectionResolver を使用して、設定された DbRef に基づいてデータベース接続を解決します。
    2.  GenericParquetExportTaskletConfig の TableName, SQLSelectColumns, SQLOrderBy を使用して、動的に SQL クエリを構築します。
    3.  `dbConn.ScanRowsToStruct` を使用して動的に `scanFunc` を生成し、`readerComponent.NewSqlCursorReader` を初期化します。
    4.  goroutine とチャネルを用いて、リーダーとライターの並行処理パイプラインを構築します。
        *   リーダー Goroutine: SqlCursorReader を使用してデータベースからデータを読み込み、チャネルに送信します。
        *   ライター Goroutine: チャネルからデータを受信し、内部の ParquetWriter の Write メソッドを呼び出します。ParquetWriter は内部でデータをバッファリングします。
    5.  両方の goroutine の完了を待ちます。

### 5.3. Close(ctx context.Context) error

*   **役割**: 使用したリソースを解放します。
*   **動作**:
    1.  Execute メソッド内で ParquetWriter の Close が defer されるため、このメソッド自体では特別なリソース解放は行いません。

### 5.4. SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error

*   **役割**: タスクレットの実行コンテキストを設定します。
*   **動作**: 提供された model.ExecutionContext を内部の stepExecutionContext フィールドに保存します。

### 5.5. GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)

*   **役割**: タスクレットの現在の実行コンテキストを取得します。
*   **動作**: 内部に保存されている stepExecutionContext を返します。

## 6. 主要な機能

*   **JSL からの動的設定**: GenericParquetExportTaskletConfig を介して、データベース接続、ストレージ接続、出力パス、バッファサイズ、圧縮タイプ、テーブル名、SELECT 句、ORDER BY 句、パーティションキーのカラム名とフォーマットを JSL
    から直接指定できます。
*   **リフレクションを用いたパーティションキーの動的生成**: `PartitionDefinitions` の JSL 設定に基づき、Go の `reflect` パッケージを使用して、実行時にアイテムオブジェクトから指定されたフィールドの値を取得し、`time.Time` または `int64` (Unix ミリ秒) として解釈し、指定されたフォーマットでパーティションキー文字列を生成します。
*   **Fx による型特化ロジックの注入**: `itemPrototype` は、`NewGenericParquetExportTaskletBuilder` を通じて Fx から注入されます。これにより、`GenericParquetExportTasklet` 自体はジェネリックなまま、特定の Go 構造体型 (`T`) に対応するデータベーススキャンと Parquet スキーマ推論が可能になります。
*   **並行処理**: SqlCursorReader と ParquetWriter を独立した goroutine で実行し、チャネルを介してデータをやり取りすることで、I/O バウンドな処理の効率を向上させます。

## 7. 設計上の考慮事項

*   **型安全性とリフレクションのバランス**: 汎用性を高めるためにリフレクションを使用していますが、リフレクションはコンパイル時の型チェックを回避するため、実行時エラーのリスクを伴います。PartitionKeyColumn のフィールド名や型が JSL と Go
    の構造体で一致しない場合、実行時エラーが発生する可能性があります。このリスクは、Fx による `itemPrototype` の注入と、`DBConnection.ScanRowsToStruct` の利用によって、主要なデータフローの型安全性を維持することで軽減しています。
*   **JSL 設定の冗長性**: SQLSelectColumns は JSL で完全な SQL SELECT 句として指定する必要があるため、JSL ファイルが冗長になる可能性があります。これは、Go の型システムと SQL の動的な性質とのトレードオフです。より高度な抽象化（例: Go
    構造体のフィールド名から自動的に SELECT 句を生成する）も可能ですが、SQL の柔軟性が失われる可能性があります。
*   **エラーハンドリング**: パイプライン内の各ステージ（DB 接続、読み込み、書き込み、Parquet 生成、アップロード）で発生するエラーは適切に捕捉され、フレームワークのエラーハンドリングメカニズムに統合されます。特に xintongsys/parquet-go
    ライブラリのパニックは defer recover() で捕捉し、エラーに変換します。
*   **メモリ使用量**: ParquetWriter は Close が呼び出されるまでデータを内部バッファに蓄積します。大量のデータを処理する場合、メモリ使用量に注意が必要です。必要に応じて、ParquetWriter
    の内部でバッファリングの閾値を設けるなどの対策を検討できます。

    ## 8. JSL (Job Specification Language) 参考例

    `GenericParquetExportTasklet` を使用する JSL (Job Specification Language) の参考例を以下に示します。この例では、`example/weather/internal/domain/entity/weather_entity.go` に定義されている `WeatherDataToStore` 構造体に対応するデータをデータベースから読み込み、Parquet 形式でストレージに出力するシナリオを想定しています。

    ### JSL 例: `export_weather_data_job.yaml`

    ```yaml
    job:
      id: exportWeatherDataToParquetJob
      name: "気象データParquetエクスポートジョブ"
      description: "データベースから気象データを読み込み、日次パーティションでParquetファイルとしてS3に出力するジョブ"

      # ジョブの実行パラメータ (オプション)
      parameters:
        exportDate: "#{systemProperties['job.launch.date']}" # 例: ジョブ起動日をパラメータとして使用

      steps:
        exportWeatherStep:
          name: "気象データエクスポートステップ"
          tasklet:
            # GenericParquetExportTasklet を参照。
            # この 'genericParquetExportTasklet' は、アプリケーションのDIコンテナ (Fxなど) で
            # NewGenericParquetExportTaskletBuilder を通じて登録されている必要があります。
            ref: genericParquetExportTasklet
            properties:
              # データベース接続の参照名
              # pkg/batch/adapter/database/config/config.go の DatabaseConfig に対応する名前
              dbRef: "weather_db"

              # ストレージ接続の参照名
              # S3やGCSなどのストレージアダプタに対応する名前
              storageRef: "s3_storage"

              # Parquetファイルの出力ベースディレクトリ
              # 例: s3://my-bucket/weather_data/daily
              outputBaseDir: "s3://my-bucket/weather_data/daily"

              # データベースからの読み込みバッファサイズ (アイテム数)
              readBufferSize: "1000"

              # Parquetファイルの圧縮タイプ (SNAPPY, GZIP, ZSTD, UNCOMPRESSEDなど)
              parquetCompressionType: "SNAPPY"

              # データを読み込むデータベースのテーブル名
              tableName: "weather_data_to_store"

              # データベースから選択するカラムのSQL SELECT句
              # WeatherDataToStore 構造体のフィールドに対応するカラム名を指定
              sqlSelectColumns: "time, weather_code, temperature_2m, latitude, longitude, collected_at"

              # データベースからデータを取得する際のORDER BY句
              sqlOrderBy: "time ASC"

              # 複数のパーティション定義
              partitionDefinitions:
                - key: "CollectedAt" # Go構造体のフィールド名
                  format: "2006-01-02" # time.Time型の場合のフォーマット
                  prefix: "dt=" # パーティションディレクトリのプレフィックス
                - key: "CollectedAt"
                  format: "15"
                  prefix: "hour="
                - key: "WeatherCode"
                  format: "%d" # int型の場合のフォーマット
                  prefix: "code="
    ```

    ### JSL の解説

    *   **`job.id`**: ジョブの一意な識別子。
    *   **`steps.exportWeatherStep.tasklet.ref`**: 使用するタスクレットの参照名です。`genericParquetExportTasklet` は、Go アプリケーションの起動時に Fx (Dependency Injection) コンテナを通じて登録される必要があります。具体的には、`NewGenericParquetExportTaskletBuilder` 関数が特定の Go 構造体型 (`WeatherDataToStore` など) とそのスキャン関数、プロトタイプインスタンスと共に登録されます。
    *   **`properties`**: `GenericParquetExportTaskletConfig` 構造体に対応する設定項目です。
        *   **`dbRef`**: データベース接続設定の名前。アプリケーションの `config.yaml` などで定義されたデータベース接続を参照します。
        *   **`storageRef`**: ストレージ接続設定の名前。S3 や GCS などのオブジェクトストレージへの接続を参照します。
        *   **`outputBaseDir`**: Parquet ファイルが出力されるベースディレクトリ（例: S3 バケットのパス）。
        *   **`readBufferSize`**: データベースから一度に読み込むレコードのバッファサイズ。
        *   **`parquetCompressionType`**: 出力される Parquet ファイルの圧縮方式。
        *   **`tableName`**: データを読み込むデータベーステーブルの名前。
        *   **`sqlSelectColumns`**: データベースから取得するカラムを指定する SQL の `SELECT` 句。`WeatherDataToStore` 構造体のフィールドに対応するカラム名を指定します。
        *   **`sqlOrderBy`**: データベースからデータを取得する際のソート順を指定する SQL の `ORDER BY` 句。
        *   **`partitionDefinitions`**: 複数のパーティションキーを定義します。
            *   **`key`**: Go構造体のフィールド名。
            *   **`format`**: フォーマット文字列。`time.Time`型の場合はGoの参照時刻レイアウト（例: "2006-01-02"）、その他の型の場合は`fmt.Sprintf`のフォーマット指定子（例: "%d"）。
            *   **`prefix`**: パーティションディレクトリ名のプレフィックス（例: "dt="）。
    この JSL を使用することで、`WeatherDataToStore` 型のデータをデータベースから読み込み、`outputBaseDir/dt=YYYY-MM-DD/hour=HH/code=XXX/` のようなパスに階層的にパーティションされた Parquet ファイルとして出力することが可能になります。
