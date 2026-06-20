# 🗺️ Surfin Component Roadmap

## 🚀 Phase 1 - Current（実装済み）

コアフレームワークと基本的なReader/Writer/Adapterが揃っています。

### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| SqlCursorReader | DB | DBレコードをカーソルでストリーミング読み取り |
| CsvCursorReader | File | CSVファイルをカーソルでストリーミング読み取り |

### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| ParquetWriter | File | Parquetファイルへの書き出し |
| SqlBulkWriter | DB | DBへのバルクインサート |

### 🔧 Tasklet

| 名前 | 用途 |
|---|---|
| GenericParquetExportTasklet | DBクエリで絞り込んだデータをストリームでParquetファイルへ書き出し |

### 🗄️ Storage Adapter

| 名前 | 用途 |
|---|---|
| StorageAdapter/local | ローカルファイルシステムへのアクセス |
| StorageAdapter/gcs | Google Cloud Storage へのアクセス |

### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/mysql | RDBMS | MySQL への接続 |
| DatabaseAdapter/postgres | RDBMS | PostgreSQL への接続 |
| DatabaseAdapter/sqlite | RDBMS | SQLite への接続 |

### 📡 Observability Adapter

| 名前 | 用途 |
|---|---|
| ObservabilityAdapter/opentelemetry | OpenTelemetry によるトレース・メトリクス統合 |
| ObservabilityAdapter/opentelemetry/metrics | Prometheus メトリクスの収集 |
| ObservabilityAdapter/opentelemetry/trace | 分散トレーシング |

### 🔐 Web Proxy Adapter

| 名前 | 用途 |
|---|---|
| WebProxyAdapter/apikey | APIキー認証 |

---

## 🔧 Phase 2 - Next（優先開発中）

Reader系を中心に、実用的なパイプラインを構築するために必要なコンポーネントを追加します。

### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| SqlPagingReader | DB | DBレコードをページ単位で読み取り（再起動との相性が良い） |
| ParquetCursorReader | File | Parquetファイルをカーソルでストリーミング読み取り |
| MultiResourceItemReader | File | 複数ファイルを横断して読み取り |
| JsonItemReader | File | JSONファイルの読み取り |

### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| CsvBulkWriter | File | CSVファイルへの一括書き出し |
| JsonBulkWriter | File | JSONファイルへの一括書き出し |

---

## 📋 Phase 3 - Planned（計画中）

データ基盤・クラウドネイティブなユースケースをカバーするコンポーネントを追加します。

### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| IcebergCursorReader | File | Icebergテーブルをカーソルでストリーミング読み取り |
| XmlItemReader | File | XMLファイルの読み取り |
| KafkaStreamReader | Stream | Kafkaトピックからストリーミング読み取り |
| PubSubStreamReader | Stream | Google Cloud Pub/Subからストリーミング読み取り |

### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| IcebergBulkWriter | File | Icebergテーブルへの一括書き出し |
| KafkaStreamWriter | Stream | Kafkaトピックへのストリーミング書き出し |
| PubSubStreamWriter | Stream | Google Cloud Pub/Subへのストリーミング書き出し |

### 🗄️ Storage Adapter

| 名前 | 用途 |
|---|---|
| StorageAdapter/s3 | Amazon S3 へのアクセス |

### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/bigquery | Data Warehouse | Google BigQuery への接続 |

### 📡 Observability Adapter

| 名前 | 用途 |
|---|---|
| ObservabilityAdapter/opentelemetry/logs | ログの収集・エクスポート |

### 🔐 Web Proxy Adapter

| 名前 | 用途 |
|---|---|
| WebProxyAdapter/oauth | OAuth 2.0 認証 |
| WebProxyAdapter/hmac | HMAC署名認証 |
| WebProxyAdapter/basic | Basic認証 |

---

## 🔮 Phase 4 - Future（需要次第）

エンタープライズ環境やレガシーシステムとの連携が必要な場合に対応します。

### 🗄️ Storage Adapter

| 名前 | 用途 |
|---|---|
| StorageAdapter/sftp | SFTPサーバーへのアクセス（レガシー環境との連携） |

### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/sqlserver | RDBMS | Microsoft SQL Server への接続 |
| DatabaseAdapter/oracle | RDBMS | Oracle Database への接続 |

---

## 💬 Feedback

ロードマップへのご意見・優先度のリクエストは GitHub Issues へ。

- **GitHub Issues**: [ご意見・ご要望](https://github.com/tigerroll/surfin/issues)
