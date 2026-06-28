# 🗺️ Surfin Component Roadmap

## 🎯 What You Can Build with Surfin

利用者向けのユースケース一覧です。「何ができるか」を確認してください。

### ✅ 今すぐできること（v0.1）

| ユースケース | 使うコンポーネント |
|---|---|
| PostgreSQL → Parquet → GCS への大量エクスポート | GenericParquetExportTasklet + StorageAdapter/gcs |
| CSV → Transform → PostgreSQL への取り込み | CsvCursorReader + SqlBulkWriter |
| DB → DB へのETL | SqlCursorReader + SqlBulkWriter |
| 障害後の途中再開 | JobRepository + ExecutionContext |
| 二重実行の防止 | 楽観的ロック（標準） |
| OpenTelemetry / Prometheus によるバッチ監視 | ObservabilityAdapter/opentelemetry |

### 🔧 まもなく使えるようになること（Phase 2）

| ユースケース | 使うコンポーネント |
|---|---|
| 大量DBレコードのページング処理 | SqlPagingReader |
| 複数ファイルを横断するETL | MultiResourceItemReader |
| Parquetファイルの読み込み | ParquetCursorReader |
| S3 へのファイル出力 | StorageAdapter/s3 |

### 🗓 将来できるようになること（Phase 3〜4）

| ユースケース | 使うコンポーネント |
|---|---|
| BigQuery への接続 | DatabaseAdapter/bigquery |
| Iceberg テーブルへの書き出し・読み取り | IcebergBulkWriter / IcebergCursorReader |
| Kafka からの読み取り・書き出し | KafkaStreamReader / KafkaStreamWriter |
| ジョブ完了・失敗の Slack / Email 通知 | NotifyTasklet + NotifierAdapter |
| PDF帳票の生成・メール配信 | PdfTemplateTasklet + NotifierAdapter/email |
| Excel ファイルの読み取り・書き出し | ExcelCursorReader / ExcelBulkWriter |
| EDI・固定長ファイルの処理 | FixedLengthCursorReader |
| Google Drive / スプレッドシート連携 | StorageAdapter/googledrive |
| NDJSON・XML・TSV の処理 | NdjsonCursorReader / XmlCursorReader / TsvCursorReader |
| SQLServer・Oracle への接続 | DatabaseAdapter/sqlserver / DatabaseAdapter/oracle |
| SFTP 経由のファイル連携 | StorageAdapter/sftp |

---

## 🔬 Component Reference（開発者向け詳細）

各コンポーネントの実装状況の詳細です。

---

### 🚀 Phase 1 - Current（実装済み）

コアフレームワークと基本的なReader/Writer/Adapterが揃っています。

#### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| SqlCursorReader | DB | DBレコードをカーソルでストリーミング読み取り |
| CsvCursorReader | File | CSVファイルをカーソルでストリーミング読み取り |

#### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| ParquetWriter | File | Parquetファイルへの書き出し |
| SqlBulkWriter | DB | DBへのバルクインサート |

#### 🔧 Tasklet

| 名前 | 用途 |
|---|---|
| GenericParquetExportTasklet | DBクエリで絞り込んだデータをストリームでParquetファイルへ書き出し |

#### 🗄️ Storage Adapter

| 名前 | 用途 |
|---|---|
| StorageAdapter/local | ローカルファイルシステムへのアクセス |
| StorageAdapter/gcs | Google Cloud Storage へのアクセス |

#### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/mysql | RDBMS | MySQL への接続 |
| DatabaseAdapter/postgres | RDBMS | PostgreSQL への接続 |
| DatabaseAdapter/sqlite | RDBMS | SQLite への接続 |

#### 📡 Observability Adapter

| 名前 | 用途 |
|---|---|
| ObservabilityAdapter/opentelemetry | OpenTelemetry によるトレース・メトリクス統合 |
| ObservabilityAdapter/opentelemetry/metrics | Prometheus メトリクスの収集 |
| ObservabilityAdapter/opentelemetry/trace | 分散トレーシング |

#### 🔐 Web Proxy Adapter

| 名前 | 用途 |
|---|---|
| WebProxyAdapter/apikey | APIキー認証 |

---

### 🔧 Phase 2 - Next（優先開発中）

Reader系を中心に、実用的なパイプラインを構築するために必要なコンポーネントを追加します。

#### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| SqlPagingReader | DB | DBレコードをページ単位で読み取り（再起動との相性が良い） |
| MultiResourceItemReader | File | 複数ファイルを横断して読み取り |
| ParquetCursorReader | File | Parquetファイルをカーソルでストリーミング読み取り |

#### 🗄️ Storage Adapter

| 名前 | 用途 |
|---|---|
| StorageAdapter/s3 | Amazon S3 へのアクセス |

---

### 📋 Phase 3 - Planned（計画中）

データ基盤・クラウドネイティブなユースケースをカバーするコンポーネントを追加します。

#### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| IcebergCursorReader | File | Icebergテーブルをカーソルでストリーミング読み取り |
| NdjsonCursorReader | File | NDJSONファイルをカーソルでストリーミング読み取り（大量データ向け） |
| TsvCursorReader | File | TSVファイルのカーソル読み取り |
| XmlCursorReader | File | XMLファイルをStAXベースでストリーミング読み取り（大量データ向け） |
| XmlItemReader | File | XMLファイルの読み取り（小〜中規模・複雑な構造向け、DOMベース） |
| JsonItemReader | File | JSONファイルの読み取り（小〜中規模・複雑な構造向け、DOMベース） |
| KafkaStreamReader | Stream | Kafkaトピックからストリーミング読み取り |
| PubSubStreamReader | Stream | Google Cloud Pub/Subからストリーミング読み取り |

#### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| IcebergBulkWriter | File | Icebergテーブルへの一括書き出し |
| CsvBulkWriter | File | CSVファイルへの一括書き出し |
| JsonBulkWriter | File | JSONファイルへの一括書き出し |
| KafkaStreamWriter | Stream | Kafkaトピックへのストリーミング書き出し |
| PubSubStreamWriter | Stream | Google Cloud Pub/Subへのストリーミング書き出し |

#### 🔧 Tasklet

| 名前 | 用途 |
|---|---|
| NotifyTasklet | ジョブの完了・失敗をNotifierAdapterを通じて通知 |
| PdfTemplateTasklet | テンプレートに値を埋め込んでPDFを生成 |

#### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/bigquery | Data Warehouse | Google BigQuery への接続 |

#### 📡 Observability Adapter

| 名前 | 用途 |
|---|---|
| ObservabilityAdapter/opentelemetry/logs | ログの収集・エクスポート |

#### 🔐 Web Proxy Adapter

| 名前 | 用途 |
|---|---|
| WebProxyAdapter/oauth | OAuth 2.0 認証（StorageAdapter/googledriveの前提） |
| WebProxyAdapter/hmac | HMAC署名認証 |
| WebProxyAdapter/basic | Basic認証 |

#### 🔔 Notifier Adapter

| 名前 | 用途 |
|---|---|
| NotifierAdapter/slack | Slackへの通知 |
| NotifierAdapter/email | メールでの通知（PDF添付対応） |
| NotifierAdapter/webhook | Webhookでの通知 |

---

### 🔮 Phase 4 - Future（需要次第）

エンタープライズ環境・レガシーシステム・特定用途向けのコンポーネントです。

#### 📖 Reader

| 名前 | 種別 | 用途 |
|---|---|---|
| FixedLengthCursorReader | File | 固定長ファイルの読み取り（EDI・レガシー基幹システム連携） |
| ExcelCursorReader | File | Excelファイルの読み取り（サプライヤーからの納品データ等） |

#### ✍️ Writer

| 名前 | 種別 | 用途 |
|---|---|---|
| ExcelBulkWriter | File | Excelファイルへの書き出し（帳票・発注書等） |

#### 🗄️ Storage Adapter

| 名前 | 用途 | 依存 |
|---|---|---|
| StorageAdapter/sftp | SFTPサーバーへのアクセス（レガシー環境との連携） | - |
| StorageAdapter/googledrive | Google Driveへのアクセス（スプレッドシート・ファイル連携） | WebProxyAdapter/oauth |

#### 🗃️ Database Adapter

| 名前 | 種別 | 用途 |
|---|---|---|
| DatabaseAdapter/sqlserver | RDBMS | Microsoft SQL Server への接続 |
| DatabaseAdapter/oracle | RDBMS | Oracle Database への接続 |

#### 🔔 Notifier Adapter

| 名前 | 用途 |
|---|---|
| NotifierAdapter/sms | SMSでの緊急通知（Twilio / AWS SNS等） |

---

## 💬 Feedback

ロードマップへのご意見・優先度のリクエストは GitHub Issues へ。

- **GitHub Issues**: [ご意見・ご要望](https://github.com/tigerroll/surfin/issues)
