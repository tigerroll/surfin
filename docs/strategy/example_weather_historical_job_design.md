# 設計ドキュメント: Historical Weather Data Job

## 1. 概要
本ドキュメントは、Open-Meteo Archive API を利用して過去の気象データを取得し、データベースへ保存する historical ジョブの設計方針を定義する。既存の forecast ジョブのアーキテクチャを継承し、コンポーネントの再利用性を最大化する。

## 2. アーキテクチャ
本ジョブは、既存の forecast ジョブと同様に **Chunk-oriented（チャンク指向）アーキテクチャ** を採用する。

- **Reader:** 新規作成 (`HistoricalWeatherReader`)。CSV形式のAPIレスポンスをストリーミング処理する。
- **Processor:** 既存の `HourlyForecastTransformProcessor` を再利用（またはパススルー）。
- **Writer:** 既存の `HourlyForecastDatabaseWriter` を再利用。

## 3. 主要な設計決定

### 3.1. 日付計算ロジック
ジョブ実行時の日付指定を柔軟かつ自動化するため、Reader 内で以下のロジックを実装する。

- **`endDate` の決定:**
  - プロパティ指定がある場合: 指定された日付を使用。
  - 省略時: `実行日 - 3日` を自動計算。
- **`startDate` の決定:**
  - プロパティ指定がある場合: 指定された日付を使用。
  - 省略時: `endDate - 7日` を自動計算。

これにより、JSL 側で日付を意識せずにジョブを実行可能とする。

### 3.2. データ取得戦略
- **WebProxyAdapter の利用:** `application.yaml` で定義された `openmeteo-historical` プロキシ設定を使用し、API 接続を抽象化する。
- **ストリーミング処理:** `CsvCursorReader` をラップすることで、API レスポンスをメモリに全展開せず、行単位で処理を行う。これにより大規模な期間のデータ取得にも耐えうる設計とする。

## 4. 設定と定義

### 4.1. プロキシ設定 (`application.yaml`)

```yaml
surfin:
  adapter:
    webproxy:
      openmeteo-historical:
        type: "NONE"
        api_endpoint: "https://archive-api.open-meteo.com"
```

### 4.2. ジョブ定義 (`historical_job.yaml`)

```yaml
id: historicalJob
flow:
  elements:
    fetchHistoricalDataStep:
      reader:
        ref: historicalWeatherReader
        properties:
          webProxyRef: "openmeteo-historical"
          # startDate/endDate は省略可能
          latitude: "52.52"
          longitude: "13.41"
      # ... (Processor/Writer は既存のものを参照)

```

## 5. マイグレーション戦略

* 既存の `hourly_forecast` テーブルを共有する場合、DDL の追加は不要。
* 履歴データ専用のテーブルが必要な場合は、`resources/migrations/postgres/002_create_historical_weather.sql` を作成し、マイグレーションタスクレットで適用する。

## 6. 実装ステップ

1. **Reader 実装:** `HistoricalWeatherReader` を作成し、日付計算ロジックと `CsvCursorReader` のラップを実装する。
2. **DI 登録:** `internal/step/reader/module.go` に Builder を追加し、Fx に登録する。
3. **JSL 作成:** `historical_job.yaml` を作成する。
4. **動作確認:** `SERVICE_NAME=historical task weather:run` で実行し、日付計算とデータ取得が正しく行われるか検証する。
