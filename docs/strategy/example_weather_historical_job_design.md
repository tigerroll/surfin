# 設計ドキュメント: Historical Weather Data Job

## 1. 概要
本ドキュメントは、Open-Meteo Archive API を利用して過去の気象データを取得し、データベースへ保存する `historyJob` の設計方針を定義する。予測データ（Forecast）と履歴データ（Historical）のドメイン境界を明確にするため、専用の Processor および Writer コンポーネントを新規作成し、保守性と拡張性を担保する。

## 2. アーキテクチャ
本ジョブは、**Chunk-oriented（チャンク指向）アーキテクチャ** を採用する。

- **Reader:** 新規作成 (`HistoricalWeatherReader`)。日付計算ロジックとCSV解析を実装する。
- **Processor:** 新規作成 (`HourlyHistoryTransformProcessor`)。履歴データ専用の変換ロジックを実装する。
- **Writer:** 新規作成 (`HourlyHistoryDatabaseWriter`)。履歴データ専用テーブルへ書き込むロジックを実装する。

## 3. 主要な設計決定

### 3.1. 日付計算ロジック
ジョブ実行時の日付指定を柔軟かつ自動化するため、Reader 内で以下のロジックを実装する。

- **`endDate` の決定:**
  - プロパティ指定がある場合: 指定された日付を使用。
  - 省略時: `実行日 - 3日` を自動計算。
- **`startDate` の決定:**
  - プロパティ指定がある場合: 指定された日付を使用。
  - 省略時: `endDate - 7日` を自動計算。

### 3.2. データ取得戦略
- **WebProxyAdapter の利用:** `application.yaml` で定義された `openmeteo-historical` プロキシ設定を使用し、API 接続を抽象化する。
- **ストリーミング処理:** `CsvCursorReader` をラップすることで、API レスポンスをメモリに全展開せず、行単位で処理を行う。

### 3.3. テーブルおよびドメインの分離戦略
Forecast（予測データ）と Historical（履歴データ）は、データライフサイクルや検索要件が異なるため、以下の分離を行う。

- **Entityの分離:** `HourlyForecastToStore`（予測用）と `HourlyHistoryToStore`（履歴用）を別々のEntityとして定義する。
- **コンポーネントの分離:** 予測データ用と履歴データ用で Processor/Writer を分離し、それぞれのドメイン要件に最適化する。
- **Historical 専用テーブル:** `hourly_history` を作成し、履歴データを格納する。

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

### 4.2. ジョブ定義 (`history_job.yaml`)

```yaml
id: historyJob
flow:
  elements:
    fetchHistoricalDataStep:
      reader:
        ref: historicalWeatherReader
        properties:
          webProxyRef: "openmeteo-historical"
          latitude: "52.52"
          longitude: "13.41"
      processor:
        ref: hourlyHistoryTransformProcessor
      writer:
        ref: hourlyHistoryDatabaseWriter
        properties:
          targetDBName: "workload"
```

## 5. マイグレーション戦略

*   **履歴データ専用テーブルの作成:**
    `resources/migrations/<db_type>/20250819113001_create_hourly_history_table.up.sql` を作成し、`hourly_history` テーブルを定義する。
*   **分離の理由:**
    *   **ライフサイクル管理:** 履歴データは長期保存、Forecast データは短期保存という運用上の分離を容易にする。
    *   **パフォーマンス:** 巨大な履歴データと頻繁に更新される Forecast データを分けることで、インデックスの肥大化を防ぎ、クエリ性能を維持する。
    *   **安全性:** 履歴データのバッチ処理による誤操作が、Forecast データに影響を与えるリスクを排除する。

## 6. 実装ステップ

1.  **マイグレーション作成:** `hourly_history` テーブル作成用 SQL を追加。
2.  **Entity作成:** `HourlyHistoryToStore` を定義。
3.  **Processor 実装:** `HourlyHistoryTransformProcessor` を作成。
4.  **Writer 実装:** `HourlyHistoryDatabaseWriter` を作成。
5.  **DI 登録:** `internal/step/processor/module.go` 等に各 Builder を追加し、Fx に登録。
6.  **JSL 作成:** `history_job.yaml` を作成。
7.  **動作確認:** `SERVICE_NAME=history` で実行し、データが `hourly_history` に格納されることを確認。
