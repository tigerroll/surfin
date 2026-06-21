# Weather Historical Job 設計

## 1. 概要
`Weather Historical Job` は、過去の気象データをデータベースから抽出し、分析基盤（Parquet形式）へエクスポートするバッチジョブです。

## 2. ジョブフロー
1. **Metadata Migration**: フレームワークのメタデータテーブルをマイグレーション。
2. **Workload Migration**: アプリケーション固有のワークロードテーブルをマイグレーション。
3. **Fetch Weather Data**: データベースから過去の気象データを読み込み。
4. **Export Hourly Forecast**: `GenericParquetExportTasklet` を使用し、データをParquet形式でストレージへ出力。

## 3. コンポーネント構成
* **Reader**: `SqlCursorReader` を使用し、`hourly_forecast` テーブルからデータをストリーミング読み込み。
* **Tasklet**: `GenericParquetExportTasklet` を使用。
    * **Partitioning**: `CollectedAt` カラムを基に `dt=YYYY-MM-DD` 形式でパーティショニング。
    * **Storage**: GCS またはローカルストレージアダプターを使用。

## 4. 設定例 (job.yaml)
```yaml
exportHourlyForecastStep:
  id: exportHourlyForecastStep
  tasklet:
    ref: hourlyForecastExportTasklet
    properties:
      dbRef: workload
      storageRef: local
      outputBaseDir: "exported_weather_data"
```
