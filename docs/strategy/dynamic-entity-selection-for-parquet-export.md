# GenericParquetExportTaskletにおける動的エンティティ選択の設計

このドキュメントは、JSL (Job Specification Language) を通じてエンティティタイプを動的に指定し、それに基づいて `GenericParquetExportTasklet` を構築する設計について説明します。

## 1. 目的

`GenericParquetExportTasklet` はジェネリック型 `T` を使用して汎用的なデータエクスポート機能を提供します。この設計の目的は、アプリケーション層で複数のエンティティタイプをエクスポートする際に、JSLの設定のみで対象エンティティを切り替えられるようにすることです。これにより、各エンティティタイプごとに個別の `ComponentBuilder` を定義する手間を省き、設定の柔軟性を高めます。

## 2. 背景

現在の `GenericParquetExportTasklet` の利用では、`example/weather/internal/component/tasklet/module.go` 内で `tasklet.NewGenericParquetExportTaskletBuilder[entity.HourlyForecast]` のように、特定のエンティティ型をハードコードして `ComponentBuilder` を提供しています。複数のエンティティを扱う場合、それぞれのエンティティ型に対応する `ComponentBuilder` を個別に定義する必要があります。

## 3. 設計の概要

このドキュメントは、`GenericParquetExportTasklet` が現在、`example/weather/internal/component/tasklet/module.go` で特定のエンティティ型 (`entity.HourlyForecast`) をハードコードして `ComponentBuilder` を提供している現状を説明します。

### 3.1. `example/weather/internal/component/tasklet/module.go` の役割

`example/weather/internal/component/tasklet/module.go` は、FxのDIコンテナに `configjsl.ComponentBuilder` を提供します。このビルダーは、`entity.HourlyForecast` 型に特化した `GenericParquetExportTasklet` のインスタンスを構築する責任を持ちます。

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
        ref: genericParquetExportTasklet # 現在の参照名
        properties:
          # entityType プロパティは現在サポートされていません。
          # エンティティタイプは Go コード (module.go) でハードコードされています。
          dbRef: "weatherDb"
          storageRef: "gcsStorage"
          outputBaseDir: "weather/hourly_forecast"
          tableName: "hourly_forecasts"
          sqlSelectColumns: "time,weather_code,temperature_2m,latitude,longitude,collected_at"
          partitionKeyColumn: "Time" # このプロパティは現在 PartitionDefinitions に置き換えられています。
          partitionKeyFormat: "2006-01-02" # このプロパティは現在 PartitionDefinitions に置き換えられています。
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true

## 5. 考慮事項

*   **エンティティタイプの変更**: 現在、エンティティタイプは `example/weather/internal/component/tasklet/module.go` で `entity.HourlyForecast` にハードコードされています。別のエンティティタイプを使用するには、この Go コードを直接変更する必要があります。
*   **型安全性**: `GenericParquetExportTasklet` はジェネリクスを使用していますが、`module.go` で特定の型に特化してインスタンス化されるため、JSL で指定される `SQLSelectColumns` や `PartitionDefinitions` などのプロパティが、ハードコードされたエンティティ型 (`entity.HourlyForecast`) と整合していることを保証する必要があります。不整合がある場合、実行時にエラーが発生する可能性があります。
