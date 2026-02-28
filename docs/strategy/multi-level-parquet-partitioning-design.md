# マルチレベルParquetパーティショニング設計

このドキュメントは、`GenericParquetExportTasklet` において、複数のパーティションキーを階層的なパスとしてParquetファイル出力に適用するための設計について説明します。

## 1. 目的

- このドキュメントは、`GenericParquetExportTasklet` が担うべきParquetファイル出力機能の最小単位としての設計を定義します。
- `GenericParquetExportTasklet` の役割は、データベースからデータを読み込み、指定されたスキーマとパーティション定義に従ってParquet形式でストレージに書き出すことです。

- 本設計では、`GenericParquetExportTasklet` が、単一のパーティションキーだけでなく、JSL (Job Specification Language) で定義された複数のパーティションキーを組み合わせて、Hiveスタイルの階層的なディレクトリ構造を生成できるように拡張します。
- これにより、データの整理とクエリの効率化を向上させます。

## 2. 背景

- 現在の `GenericParquetExportTasklet` は、`PartitionKeyColumn` と `PartitionKeyFormat` を使用して単一のパーティションキーのみをサポートしています。
- しかし、データウェアハウスのプラクティスでは、日付、時刻、カテゴリなど複数の属性に基づいてデータをパーティション分割することが一般的であり、これによりデータの管理と分析の柔軟性が高まります。

## 3. 設計の概要

- 既存の `GenericParquetExportTasklet` の設定とパーティションキー生成ロジックを拡張し、複数のパーティション定義をサポートします。

### 3.1. 設定構造体の変更

`pkg/batch/component/tasklet/generic/parquet_export_tasklet.go` 内の `GenericParquetExportTaskletConfig` 構造体を以下のように変更します。

*   既存の `PartitionKeyColumn` および `PartitionKeyFormat` フィールドを削除します。
*   複数のパーティション定義を保持するための新しいフィールド `PartitionDefinitions []PartitionDefinition` を追加します。
*   `PartitionDefinition` は、各パーティションキーのフィールド名 (`Key`) と、そのフォーマット (`Format`) を定義する新しい構造体です。

```go
// pkg/batch/component/tasklet/generic/parquet_export_tasklet.go

// GenericParquetExportTaskletConfig holds the configuration for [GenericParquetExportTasklet].
type GenericParquetExportTaskletConfig struct {
    // ... (既存のフィールドは変更なし) ...
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

### 3.2. `partitionKeyFunc` の再構築

`NewGenericParquetExportTasklet` 関数内で `partitionKeyFunc` を生成するロジックを修正します。

*   `partitionKeyFunc` は、`GenericParquetExportTaskletConfig.PartitionDefinitions` の各要素を順番に処理します。
*   各 `PartitionDefinition` について、リフレクションを使用して `item` から `Key` で指定されたフィールドの値を取得します。
*   取得した値は、`Format` と `Prefix` に従って文字列に変換されます。
    *   `time.Time` 型の場合、`Format` はGoの参照時刻レイアウトとして解釈されます。
    *   その他の型の場合、`Format` は `fmt.Sprintf` のフォーマット指定子として使用されます。`Format` が空の場合は、値のデフォルトの文字列表現が使用されます。
    *   `Prefix` が指定されている場合、`Prefix + formattedValue` の形式でパーティション名が生成されます。`Prefix` が空の場合、`Key` が小文字に変換されてプレフィックスとして使用されます（例: `key=value`）。
*   生成された各パーティション文字列は `/` で結合され、最終的なHiveスタイルのパーティションパス（例: `dt=YYYY-MM-DD/tm=HH-MM-SS/status=VALUE`）が作成されます。

### 3.3. JSLでの設定例

この設計を導入することで、JSLでは以下のようにタスクレットを定義できるようになります。

```yaml
# JSLの例
flow:
  start-element: exportDataStep
  elements:
    exportDataStep:
      id: exportDataStep
      tasklet:
        ref: dynamicParquetExportTasklet # GenericParquetExportTaskletBuilder のタグ名
        properties:
          entityType: "HourlyForecast"
          dbRef: "weatherDb"
          storageRef: "gcsStorage"
          outputBaseDir: "weather/hourly_forecast"
          tableName: "hourly_forecasts"
          sqlSelectColumns: "time,weather_code,temperature_2m,latitude,longitude,collected_at"
          # 変更点: PartitionDefinitions を使用
          partitionDefinitions:
            - key: "CollectedAt"
              format: "2006-01-02" # time.Time から日付部分を抽出
              prefix: "dt="       # 例: dt=2023-10-27
            - key: "CollectedAt"
              format: "15"        # time.Time から時間（時）部分を抽出
              prefix: "hour="     # 例: hour=10
            - key: "WeatherCode"
              format: "%d"        # int 型の値をそのまま使用
              prefix: "code="     # 例: code=100
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true
```

- この設定により、例えば `weather/hourly_forecast/dt=2023-10-27/hour=10/code=100/data_....parquet` のようなパスが生成されるようになります。
- `partitionDefinitions` に単一の定義のみを含めれば、単一のパーティションキーとして機能します。

## 4. 考慮事項

この修正を実装する際には、以下の点に特に注意が必要です。

*   **フィールドの型とフォーマットの整合性**:
    *   `partitionKeyFunc` は、`PartitionDefinition.Key` で指定されたフィールドの実際のGoの型（`time.Time`, `int`, `string` など）を適切に識別し、それに応じたフォーマット処理を行う必要があります。
    *   `Format` 文字列が、フィールドの型と互換性があることを保証する必要があります。例えば、`time.Time` 以外の型にGoの参照時刻レイアウトを適用しようとすると、予期せぬ結果やエラーが発生します。
    *   `time.Time` のフォーマットにはGoの参照時刻レイアウト (`2006-01-02 15:04:05`) を使用し、他の型には `fmt.Sprintf` のフォーマット指定子 (`%s`, `%d` など) を使用するロジックを実装する必要があります。
*   **リフレクションによるフィールドアクセス時のエラーハンドリング**:
    *   `PartitionDefinition.Key` で指定されたフィールドが `itemPrototype` に存在しない場合、`reflect.Value.IsValid()` をチェックし、適切なエラーを返す必要があります。
    *   フィールド名の大文字・小文字の区別（Goのリフレクションは区別します）に注意し、JSLでの指定とGoの構造体フィールド名が一致していることを確認する必要があります。
*   **`PartitionDefinitions` の設定ミス**:
    *   JSL側で `PartitionDefinitions` が空の場合や、無効な定義が含まれている場合の動作を考慮し、必要に応じてバリデーションやデフォルト動作を定義します。
*   **Goの `time.Format` の参照時刻ルール**:
    *   `time.Time` のフォーマットには、`"2006-01-02"` や `"15"` のようなGo独自の参照時刻レイアウトを使用する必要があります。他の言語のような `YYYY-MM-DD` や `HH` といった記号はGoではリテラル文字列として扱われるため、注意が必要です。

## 5. 影響範囲

この修正によって影響を受ける既存のコードは以下の通りです。

*   `pkg/batch/component/tasklet/generic/parquet_export_tasklet.go`
    *   `GenericParquetExportTaskletConfig` 構造体の定義
    *   `GenericParquetExportTasklet` 構造体（`partitionKeyFunc` の型定義は変更なし）
    *   `NewGenericParquetExportTasklet` 関数（`partitionKeyFunc` の生成ロジック、`mapstructure` のデコード設定）
    *   `NewGenericParquetExportTaskletBuilder` 関数（`mapstructure` のデコード設定）
*   `example/weather/internal/component/tasklet/module.go`
    *   `Module` 変数（`fx.Annotate` 内の `NewGenericParquetExportTaskletBuilder` の呼び出し部分。引数の変更はないが、内部で処理されるプロパティの構造が変わるため、論理的に影響を受ける）
*   `example/weather/internal/domain/entity/weather_entity.go`
    *   `HourlyForecast` 構造体、`WeatherDataToStore` 構造体（パーティションキーとして使用されるフィールドの型が、`partitionKeyFunc` で適切に処理されることを確認するため）
