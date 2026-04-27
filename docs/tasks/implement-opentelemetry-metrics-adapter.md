# OpenTelemetry Metrics Adapter 実装計画

## 1. 概要

Surfinバッチフレームワークにおいて、OpenTelemetry Metrics Adapter (`pkg/batch/adapter/metrics/opentelemetry`) を実装する計画です。これにより、`pkg/batch/core/metrics.MetricRecorder`
インターフェースをOpenTelemetry Go SDKを用いて具体的に実装し、Surfinが生成するメトリクスをOTLP互換の様々なバックエンドへエクスポート可能にします。

| タスク                                                                | ステータス | 完了予定日 |
| :-------------------------------------------------------------------- | :--------- | :--------- |
| `pkg/batch/adapter/metrics/opentelemetry/config.go` の実装            | 未着手     |            |
| `pkg/batch/adapter/metrics/opentelemetry/recorder.go` の実装          | 未着手     |            |
| `pkg/batch/adapter/metrics/opentelemetry/provider.go` の実装          | 未着手     |            |
| `pkg/batch/adapter/metrics/opentelemetry/module.go` の実装            | 未着手     |            |
| ユニットテストの作成 (`test/pkg/batch/adapter/metrics/opentelemetry`) | 未着手     |            |
| 統合テストの作成 (`test/pkg/batch/adapter/metrics/opentelemetry`)     | 未着手     |            |

## 2. 実装タスク

### 2.1. `pkg/batch/adapter/metrics/opentelemetry/config.go` の実装

*   `OTLPExporterConfig` 構造体を定義し、OTLPエクスポーター（gRPC/HTTP）に必要な設定項目（ホスト、ポート、プロトコル、認証ヘッダー、タイムアウト、圧縮方式、TLS設定など）を含める。
*   `PrometheusPushgatewayConfig` 構造体を定義するが、OpenTelemetry SDKの直接の範囲外であるため、実装スコープ外であることをコメントで明記する。

### 2.2. `pkg/batch/adapter/metrics/opentelemetry/recorder.go` の実装

*   `OtelMetricRecorder` 構造体を定義し、`pkg/batch/core/metrics.MetricRecorder` インターフェースを実装する。
*   内部で `go.opentelemetry.io/otel/metric` パッケージの `Meter` を初期化する。
*   カウンター (`Int64Counter`) やヒストグラム (`Float64Histogram`) などのOpenTelemetryメトリクスインストゥルメントを定義する。
*   `RecordJobStart`, `RecordJobEnd`, `RecordStepStart`, `RecordStepEnd`, `RecordItemRead`, `RecordItemProcess`, `RecordItemWrite`, `RecordItemSkip`, `RecordItemRetry`, `RecordChunkCommit`, `RecordDuration`
の各メソッドを実装する。
*   各メソッドにおいて、バッチ実行のコンテキスト情報（ジョブ名、ステップ名、ID、ステータスなど）をOpenTelemetryの属性 (`attribute.KeyValue`) としてメトリクスに付与するロジックを実装する。

### 2.3. `pkg/batch/adapter/metrics/opentelemetry/provider.go` の実装

*   `NewMetricRecorderProvider` 関数を実装する。
    *   `application.yaml` の `surfin.adapter.metrics` 設定を読み込む。
    *   設定された各エクスポーター（`type: otlp` のもの）に対して、`go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` または `otlpmetrichttp`
を使用してOpenTelemetryエクスポーターインスタンスを動的に作成する。
    *   複数のエクスポーターが設定されている場合、`metric.NewMultiExporter` を使用してそれらを統合する。
    *   `metric.NewPeriodicReader` を使用して、メトリクスを定期的にエクスポートするメカニズムを設定する。
    *   `metric.NewMeterProvider` を使用して `MeterProvider` を構築し、アプリケーションのリソース属性（サービス名、バージョンなど）を設定する。
    *   `OtelMetricRecorder` のインスタンスを生成し、`MeterProvider` を渡す。
    *   設定が有効でない場合やエクスポーターが設定されていない場合は、`core/metrics.NoopMetricRecorder` を返すロジックを実装する。
*   `ShutdownMeterProvider` 関数を実装し、`MeterProvider` のシャットダウン処理を行う。

### 2.4. `pkg/batch/adapter/metrics/opentelemetry/module.go` の実装

*   Fx (Go.uber.org/fx) モジュールを定義する。
*   `fx.Provide` を使用して `NewMetricRecorderProvider` 関数をプロバイダーとして登録する。
*   `fx.Invoke` と `fx.Hook` を使用して、アプリケーションのシャットダウン時に `ShutdownMeterProvider` 関数が確実に呼び出されるようにライフサイクルに登録する。

## 3. 考慮事項と対応

*   **OpenTelemetry SDK の設定**: `MeterProvider` の初期化時に、サービス名、バージョンなどのリソース属性を適切に設定する。
*   **エクスポート間隔**: `PeriodicExportingMetricReader` のエクスポート間隔を、メトリクスの鮮度とシステム負荷のバランスを考慮して設定可能にする。
*   **エラーハンドリング**: エクスポーターの初期化やメトリクスエクスポート中に発生するエラーは、適切にロギングし、アプリケーションの主要な処理を妨げないように堅牢なエラーハンドリングを実装する。
*   **パフォーマンス**: OpenTelemetry SDKのバッファリングや非同期処理の特性を理解し、適切に利用することで、メトリクス収集によるパフォーマンス影響を最小限に抑える。

## 4. テスト計画

*   ユニットテスト: 各コンポーネント（`recorder.go`, `provider.go`）の機能が期待通りに動作することを確認する。
*   ユニットテスト: `test/pkg/batch/adapter/metrics/opentelemetry` ディレクトリ配下に、各コンポーネント（`recorder.go`, `provider.go`）の機能が期待通りに動作することを確認するテストを作成する。
*   統合テスト: `test/pkg/batch/adapter/metrics/opentelemetry` ディレクトリ配下に、OpenTelemetry CollectorをDockerなどで起動し、実際にメトリクスがエクスポートされ、Collectorで受信できることを確認するテストを作成する。

## 5. 完了基準

*   上記全てのタスクが完了し、コードがレビューされ、承認されること。
*   ユニットテストおよび統合テストが成功すること。
*   関連するドキュメント（特に `docs/strategy/surfin-metrics-adapter-opentelemetry-design.md`）が、実装内容と一致していることを確認すること。
