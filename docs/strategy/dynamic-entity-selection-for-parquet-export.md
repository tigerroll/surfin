# GenericParquetExportTaskletにおける動的エンティティ選択の設計

このドキュメントは、JSL (Job Specification Language) を通じてエンティティタイプを動的に指定し、それに基づいて `GenericParquetExportTasklet` を構築する設計について説明します。

## 1. 目的

`GenericParquetExportTasklet` はジェネリック型 `T` を使用して汎用的なデータエクスポート機能を提供します。この設計の目的は、アプリケーション層で複数のエンティティタイプをエクスポートする際に、JSLの設定のみで対象エンティティを切り替えられるようにすることです。これにより、各エンティティタイプごとに個別の `ComponentBuilder` を定義する手間を省き、設定の柔軟性を高めます。

## 2. 背景

従来の `GenericParquetExportTasklet` の利用では、`example/weather/internal/component/tasklet/module.go` 内で `tasklet.NewGenericParquetExportTaskletBuilder[entity.HourlyForecast]` のように、特定のエンティティ型をハードコードして `ComponentBuilder` を提供していました。複数のエンティティを扱う場合、それぞれのエンティティ型に対応する `ComponentBuilder` を個別に定義する必要がありました。

## 3. 設計の概要

この設計では、`example/weather/internal/component/tasklet/module.go` が提供する `ComponentBuilder` を拡張し、JSLのプロパティからエンティティタイプを動的に解決するようにします。

### 3.1. `example/weather/internal/component/tasklet/module.go` の役割

`example/weather/internal/component/tasklet/module.go` は、FxのDIコンテナに `configjsl.ComponentBuilder` を提供します。このビルダーは、JSLから渡されるプロパティに基づいて `GenericParquetExportTasklet` のインスタンスを構築する責任を持ちます。

### 3.2. `ComponentBuilder` 内での `entityType` プロパティの利用

`ComponentBuilder` の実装内で、JSLから渡される `properties` マップから `entityType` というキーの値を読み取ります。この `entityType` の値が、エクスポート対象のエンティティタイプを文字列で指定します。

### 3.3. `switch` 文によるエンティティプロトタイプの動的選択

読み取った `entityType` の値に基づいて、Goの `switch` 文を使用し、対応するエンティティのゼロ値インスタンス（プロトタイプ）を決定します。このプロトタイプインスタンスは、`GenericParquetExportTasklet` のコンストラクタに渡され、Parquetスキーマの推論やパーティションキー関数の生成に利用されます。

```go
// 簡略化した例
var itemPrototype any
switch entityType {
case "HourlyForecast":
    itemPrototype = &entity.HourlyForecast{}
case "WeatherDataToStore":
    itemPrototype = &entity.WeatherDataToStore{}
default:
    // 未サポートのエンティティタイプに対するエラーハンドリング
    return nil, fmt.Errorf("unsupported entityType: %s", entityType)
}
```

### 3.4. `GenericParquetExportTasklet[any]` のインスタンス化

`GenericParquetExportTasklet` はジェネリック型 `T` を持つため、動的に決定した `itemPrototype` を使用して `tasklet.NewGenericParquetExportTasklet[any](...)` の形式でインスタンス化します。これにより、`GenericParquetExportTasklet` は `any` 型として扱われつつ、内部的には `itemPrototype` の具体的な型情報に基づいて動作します。

### 3.5. `example/weather/internal/app/module.go` の更新

`example/weather/internal/app/module.go` では、`example/weather/internal/component/tasklet/module.go` が提供する `ComponentBuilder` を `JobFactory` に登録する際に、新しい汎用的なタグ名（例: `name:"dynamicParquetExportTasklet"`）を使用するように更新します。

## 4. JSLでの利用例

この設計を導入することで、JSLでは以下のようにタスクレットを定義できるようになります。

```yaml
# JSLの例
flow:
  start-element: exportDataStep
  elements:
    exportDataStep:
      id: exportDataStep
      tasklet:
        ref: dynamicParquetExportTasklet # 新しい汎用的な参照名
        properties:
          entityType: "HourlyForecast"    # JSLプロパティでエンティティタイプを指定
          dbRef: "weatherDb"
          storageRef: "gcsStorage"
          outputBaseDir: "weather/hourly_forecast"
          tableName: "hourly_forecasts"
          sqlSelectColumns: "time,weather_code,temperature_2m,latitude,longitude,collected_at"
          partitionKeyColumn: "Time"
          partitionKeyFormat: "2006-01-02"
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true
```

別のエンティティタイプをエクスポートしたい場合は、`entityType` プロパティの値を変更するだけで対応できます。

```yaml
# JSLの別の例
flow:
  start-element: exportOtherDataStep
  elements:
    exportOtherDataStep:
      id: exportOtherDataStep
      tasklet:
        ref: dynamicParquetExportTasklet
        properties:
          entityType: "WeatherDataToStore" # こちらのエンティティタイプに変更
          dbRef: "weatherDb"
          storageRef: "gcsStorage"
          outputBaseDir: "weather/data_to_store"
          tableName: "weather_data_to_store"
          sqlSelectColumns: "time,weather_code,temperature_2m,latitude,longitude,collected_at"
          partitionKeyColumn: "CollectedAt"
          partitionKeyFormat: "2006-01-02"
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true
```

## 5. 考慮事項

*   **新しいエンティティタイプの追加**: 新しいエンティティタイプをサポートする場合、`example/weather/internal/component/tasklet/module.go` 内の `switch` 文にそのエンティティタイプと対応するプロトタイプインスタンスを追加する必要があります。
*   **型安全性**: `GenericParquetExportTasklet[any]` を使用することで、コンパイル時の厳密な型チェックの一部が `any` によって緩和されます。しかし、内部的には `itemPrototype` を通じてリフレクションが使用されるため、実行時に型不一致によるエラーが発生する可能性があります。JSLで指定される `entityType` と、`SQLSelectColumns` や `PartitionKeyColumn` などのプロパティが、選択されたエンティティ型と整合していることを保証する必要があります。
*   **エラーハンドリング**: 未サポートの `entityType` が指定された場合や、プロトタイプインスタンスの生成に失敗した場合に、適切なエラーが返されるように実装されています。
