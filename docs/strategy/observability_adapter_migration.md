# Observability Adapter への移行戦略

## 1. 目的

本ドキュメントは、既存のメトリクス収集機能（Metrics Adapter）を、トレース機能と統合された「Observability Adapter」へと移行するための戦略を記述します。この移行の主な目的は以下の通りです。

*   **機能の一元化**: メトリクスとトレースという2つの主要な可観測性シグナルを、OpenTelemetry を介して一元的に管理・設定できるようにします。
*   **設定の簡素化と柔軟性**: `application.yaml` における可観測性関連の設定を統合し、単一のエクスポーター定義でメトリクスとトレースの両方を制御できるようにします。また、必要に応じて個別の設定も可能とします。
*   **名称の統一**: システム内の可観測性関連コンポーネントの名称を「Observability」で統一し、概念的な整合性を高めます。
*   **将来的な拡張性**: OpenTelemetry 以外の可観測性プロバイダー（例: Prometheus）の追加を容易にするため、プロバイダーごとの独立した実装構造を確立します。

## 2. 変更の概要

この移行は、主に以下の3つの側面で構成されます。

1.  **ディレクトリ構造の変更**: `pkg/batch/adapter/metrics` および `pkg/batch/adapter/trace` パッケージを `pkg/batch/adapter/observability` 配下に統合します。
2.  **設定構造の変更**: `application.yaml` におけるメトリクスとトレースの設定を `surfin.adapter.observability` の下に統合し、共通のエクスポーター定義を持つようにします。各エクスポーターは、`trace` または `metrics` のサブセクションの有無によって、その機能の有効/無効を判断します。
3.  **Go コードの変更**: 新しい設定構造をパースし、OpenTelemetry の MeterProvider および TracerProvider を適切に初期化するためのロジックを調整します。

## 3. 詳細な変更点

### 3.1. ディレクトリ構造の変更

既存の `metrics` および `trace` 関連のファイルは、以下の新しいパスに移動されます。ここで `opentelemetry` は特定の可観測性プロバイダー（この場合は OpenTelemetry）の実装を格納するディレクトリを示します。

*   `pkg/batch/adapter/metrics/config/config.go` → `pkg/batch/adapter/observability/opentelemetry/metrics/config/config.go`
*   `pkg/batch/adapter/metrics/opentelemetry/module.go` → `pkg/batch/adapter/observability/opentelemetry/metrics/module.go`
*   `pkg/batch/adapter/metrics/opentelemetry/provider.go` → `pkg/batch/adapter/observability/opentelemetry/metrics/provider.go`
*   `pkg/batch/adapter/metrics/opentelemetry/recorder.go` → `pkg/batch/adapter/observability/opentelemetry/metrics/recorder.go`
*   `pkg/batch/adapter/trace/config/config.go` → `pkg/batch/adapter/observability/opentelemetry/trace/config/config.go`
*   `pkg/batch/adapter/trace/opentelemetry/module.go` → `pkg/batch/adapter/observability/opentelemetry/trace/module.go`
*   `pkg/batch/adapter/trace/opentelemetry/provider.go` → `pkg/batch/adapter/observability/opentelemetry/trace/provider.go`

また、以下の新しいファイルが作成されます。

*   `pkg/batch/adapter/observability/config/config.go`: 可観測性全般の共通設定を定義します。
*   `pkg/batch/adapter/observability/module.go`: アプリケーション設定から可観測性設定全体を抽出する Fx モジュールです。

### 3.2. 設定ファイル (`example/weather/cmd/weather/resources/application.yaml`) の変更

* `surfin.adapter` 配下の `metrics` セクションは削除され、`observability` セクションが新設されます。
* この `observability` セクションは、エクスポーターのマップを持ち、各エクスポーターは `type`、`enabled`、そしてオプションで `trace` および `metrics` の OTLP 設定を持ちます。

**変更前:**

```yaml
surfin:
  adapter:
    metrics:
      otlp_exporter:
        type: "otlp"
        enabled: true
        otlp:
          host: "otel-collector"
          port: 4317
          protocols: "grpc"
          timeout: "5s"
          insecure: true
```

**変更後:**

```yaml
surfin:
  adapter:
    observability:
      otlp_exporter: # エクスポーター名
        type: "otlp"
        enabled: true # このエクスポーター全体を有効化
        trace: # traceセクションが存在すればトレースを有効化
          host: "otel-collector"
          port: 4317
          protocols: "grpc"
          timeout: "5s"
          insecure: true
        metrics: # metricsセクションが存在すればメトリクスを有効化
          host: "otel-collector"
          port: 4317
          protocols: "grpc"
          timeout: "5s"
          insecure: true
```

* この新しい設計では、`trace` または `metrics` のサブセクションが設定ファイルに存在しない場合、その機能は自動的に無効と判断されます。
* これにより、設定の簡潔さと柔軟性が向上します。

### 3.3. Go コードの変更

#### 3.3.1. `pkg/batch/adapter/observability/config/config.go`

*   `OTLPExporterConfig` は、`trace` および `metrics` のサブセクションで使用される共通の OTLP 設定構造体として定義されます。
*   `CommonExporterConfig` は、`type`、`enabled`、そしてオプションの `Trace *OTLPExporterConfig` および `Metrics *OTLPExporterConfig` フィールドを持つ、汎用的なエクスポーター設定構造体として定義されます。
*   `ObservabilityConfig` は、`map[string]CommonExporterConfig` として定義され、`application.yaml` の `observability` セクション直下のエクスポーターマップに対応します。

#### 3.3.2. `pkg/batch/adapter/observability/module.go`

*   `NewObservabilityConfigFromAppConfig` 関数が追加され、`coreConfig.Config` から `surfin.adapter.observability` セクションを `ObservabilityConfig` 型にデコードします。
*   このモジュールは、Fx アプリケーションに `ObservabilityConfig` を提供します。

#### 3.3.3. `pkg/batch/adapter/observability/metrics/config/config.go` および `pkg/batch/adapter/observability/trace/config/config.go`

*   それぞれの `MetricsExportersConfig` および `TraceExportersConfig` は、`map[string]observability.config.CommonExporterConfig` として再定義されます。

#### 3.3.4. `pkg/batch/adapter/observability/opentelemetry/metrics/module.go` および `pkg/batch/adapter/observability/opentelemetry/trace/module.go`

*   `NewMetricsExportersConfigFromObservabilityConfig` および `NewTraceExportersConfigFromObservabilityConfig` 関数が更新され、`observability.Config` から、`enabled` かつ `Metrics` (または `Trace`) フィールドが `nil` でないエクスポーターのみをフィルタリングして返します。

#### 3.3.5. `pkg/batch/adapter/observability/opentelemetry/metrics/provider.go` および `pkg/batch/adapter/observability/opentelemetry/trace/provider.go`

*   `NewMetricRecorderProvider` および `NewTracerProvider` 関数が更新され、渡された `metricsConfig.MetricsExportersConfig` (または `traceConfig.TraceExportersConfig`) の各エクスポーターの `Metrics` (または `Trace`) フィールドから OTLP 設定を取得して、OpenTelemetry のエクスポーターを構築します。

## 4. 移行手順

開発者は、以下の手順でこの移行を適用できます。

1.  **ファイルの移動**:
    *   `pkg/batch/adapter/metrics` ディレクトリの内容を `pkg/batch/adapter/observability/opentelemetry/metrics` へ移動します。
2.  **新規ファイルの作成**:
    *   `pkg/batch/adapter/observability/config/config.go` を新規作成し、上記「3.3.1」の内容を記述します。
    *   `pkg/batch/adapter/observability/module.go` を新規作成し、上記「3.3.2」の内容を記述します。
3.  **Go コードの修正**:
    *   移動したファイル (`pkg/batch/adapter/observability/opentelemetry/...`) 内の `package` 宣言を、新しいパスに合わせて修正します（例: `package metricsconfig` や `package opentelemetry`）。
    *   すべての Go ファイルで、`import` パスを新しいディレクトリ構造に合わせて修正します。特に、`observability/config` パッケージからのインポートに注意してください。
    *   `pkg/batch/adapter/observability/opentelemetry/metrics/provider.go` および `pkg/batch/adapter/observability/opentelemetry/trace/provider.go` 内で、OTLP 設定にアクセスする際に `exporterCfg.Metrics` または `exporterCfg.Trace` を使用するように修正します。
4.  **`application.yaml` の更新**:
    *   `example/weather/cmd/weather/resources/application.yaml` を開き、`surfin.adapter.metrics` セクションを削除し、上記「3.2」の「変更後」の例に従って `surfin.adapter.observability` セクションを追加します。
5.  **Fx アプリケーションの更新**:
    *   アプリケーションのメイン Fx モジュール（通常は `main.go` またはアプリケーションのルートモジュール）で、`pkg/batch/adapter/observability` パッケージの `Module` をインポートし、Fx アプリケーションの `fx.Options` に追加します。これにより、可観測性設定がロードされ、プロバイダが初期化されます。

## 5. 影響範囲

*   **既存のメトリクス収集**: 新しい設定構造と Go コードの変更が正しく適用されれば、既存のメトリクス収集機能は引き続き OpenTelemetry を介して動作します。
*   **トレース機能の追加**: 新しい設定を有効にすることで、OpenTelemetry を使用したトレース機能がアプリケーションに追加されます。
*   **設定の互換性**: 以前の `metrics` および `trace` の設定形式は非推奨となり、新しい `observability` 形式に移行する必要があります。
*   **コードのクリーンアップ**: 移行後、古い `pkg/batch/adapter/metrics` ディレクトリは削除できます。
