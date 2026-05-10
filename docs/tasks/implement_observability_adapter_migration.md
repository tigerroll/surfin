# Observability Adapter 移行実装計画 (高レベル)

## 目的

本ドキュメントは、`docs/strategy/observability_adapter_migration.md` で定義されたObservability Adapterへの移行戦略を、より高レベルな視点からガイドするためのものです。各実装フェーズの目的、主要な変更点、およびその影響を明確にすることで、開発者が効率的かつ体系的に移行を進められるようにします。

## 主要な変更フェーズ

移行は以下の3つの主要なフェーズに分けて進行します。

### フェーズ1: ディレクトリ構造の再編成と共通設定の導入

このフェーズでは、可観測性関連のコードと設定を一元化し、将来的な拡張性を考慮した基盤を構築します。

#### 目的
*   メトリクスとトレースのコードを `observability` 配下に統合し、論理的な構造を確立する。
*   メトリクスとトレースで共通利用できる設定構造を定義し、設定の重複を排除する。
*   将来的な拡張性: OpenTelemetry 以外の可観測性プロバイダー（例: Prometheus）の追加を容易にするため、プロバイダーごとの独立した実装構造を確立する。
*   **設計原則**: 既存のDatabase AdapterやStorage Adapterの実装パターンに倣い、`pkg/batch/core/config` パッケージが特定のアダプター設定構造体（例: `observability.config.ObservabilityConfig`）に直接依存しない「逆依存」を避ける設計とします。`pkg/batch/core/config/config.go` の `SurfinConfig.AdapterConfigs` は `interface{}` のままとし、各アダプターはそれぞれ独自の設定構造体を定義し、Fxの`fx.Provide`を通じて`coreConfig.Config`から自身に必要な設定を抽出します。

#### 主要な変更点

1.  **新しいディレクトリ構造の作成**:
    *   `pkg/batch/adapter/observability` ディレクトリとそのサブディレクトリ (`metrics`, `trace`, `config`) を作成します。
2.  **既存メトリクス関連ファイルの移動**:
    *   `pkg/batch/adapter/metrics` 配下のすべてのファイルを、対応する `pkg/batch/adapter/observability/metrics` 配下へ移動します。
    *   移動後、各ファイルの `package` 宣言と `import` パスを新しい場所に合わせて修正します。
3.  **共通Observability設定の新規作成**:
    *   `pkg/batch/adapter/observability/config/config.go` を新規作成し、以下の共通設定構造体を定義します。
        *   `OTLPExporterConfig`: OTLPエクスポーターの共通設定（ホスト、ポート、プロトコルなど）。
        *   `CommonExporterConfig`: 各エクスポーターの共通設定（タイプ、有効/無効、および `Trace *OTLPExporterConfig`、`Metrics *OTLPExporterConfig`）。
        *   `ObservabilityConfig`: 全エクスポーター設定を保持するマップ (`map[string]CommonExporterConfig`)。
4.  **Observabilityモジュールの新規作成**:
    *   `pkg/batch/adapter/observability/module.go` を新規作成し、アプリケーションの全体設定 (`coreConfig.Config`) から `ObservabilityConfig` を抽出・提供するFxモジュールを定義します。
5.  **トレース関連ファイルの新規作成**:
    *   `pkg/batch/adapter/observability/opentelemetry/trace/config/config.go` を新規作成し、トレースエクスポーター設定の型を定義します（`map[string]observability.config.CommonExporterConfig` を参照）。**ここで `opentelemetry` は特定の可観測性プロバイダーの実装を格納するディレクトリを示します。**
    *   `pkg/batch/adapter/observability/opentelemetry/trace/module.go` を新規作成し、`ObservabilityConfig` からトレース関連設定をフィルタリングし、TracerProviderを提供するFxモジュールを定義します。**ここでも `opentelemetry` は特定の可観測性プロバイダーの実装を格納するディレクトリを示します。**
    *   `pkg/batch/adapter/observability/opentelemetry/trace/provider.go` を新規作成し、OpenTelemetry TracerProviderの初期化ロジック（OTLPエクスポーターの構築など）を実装します。**同様に `opentelemetry` は特定の可観測性プロバイダーの実装を格納するディレクトリを示します。**

#### 影響
*   コードベースの整理とモジュール化が進み、可観測性機能の管理が容易になります。
*   設定構造が階層化され、メトリクスとトレースで共通の設定を共有できるようになります。

### フェーズ2: 設定ファイルの統合とGoコードの適応

このフェーズでは、`application.yaml` の設定を新しい共通構造に移行し、それに合わせてGoコードを修正します。

#### 目的
*   `application.yaml` から古い `metrics` セクションを削除し、`observability` セクションに統合する。
*   Goコードが新しい `ObservabilityConfig` 構造体からメトリクスおよびトレースの設定を正しく読み込み、OpenTelemetryコンポーネントを初期化できるようにする。

#### 主要な変更点

1.  **`application.yaml` の更新**:
    *   `example/weather/cmd/weather/resources/application.yaml` から `surfin.adapter.metrics` セクションを削除します。
    *   `surfin.adapter.observability` セクションを新規追加し、`trace` と `metrics` のサブセクションを持つ共通のエクスポーター定義（例: `otlp_exporter`）を記述します。
2.  **メトリクス設定のGoコード修正**:
    *   `pkg/batch/adapter/observability/metrics/config/config.go`: `MetricsConfig` の定義を `map[string]observability.config.CommonExporterConfig` に変更し、古い `OTLPExporterConfig` などの定義を削除します。
    *   `pkg/batch/adapter/observability/opentelemetry/metrics/module.go`:
        *   `NewMetricsConfigFromAppConfig` 関数を `NewMetricsExportersConfigFromObservabilityConfig` にリネームし、引数を `*observability.config.ObservabilityConfig` に変更します。
        *   この関数内で、`ObservabilityConfig` から `enabled` かつ `Metrics` フィールドが `nil` でないエクスポーターのみをフィルタリングして返します。
    *   `pkg/batch/adapter/observability/opentelemetry/metrics/provider.go`:
        *   `NewMetricRecorderProvider` 関数の引数型を新しい `metricsConfig.MetricsConfig` (実体は `map[string]observability.config.CommonExporterConfig`) に合わせます。
        *   `createOTLPExporter` 関数が `observability.config.OTLPExporterConfig` を引数として受け取るように変更し、エクスポーター設定へのアクセスを `adapterCfg.Metrics` 経由で行うように修正します。
3.  **トレース設定のGoコード実装**:
    *   `pkg/batch/adapter/observability/opentelemetry/trace/module.go` および `provider.go` 内で、`observability.config.CommonExporterConfig` を利用してTracerProviderを構築するロジックを実装します。特に、`adapterCfg.Trace` フィールドからOTLP設定を取得するようにします。

#### 影響
*   設定ファイルがより簡潔になり、メトリクスとトレースの設定が論理的にグループ化されます。
*   Goコードが新しい設定構造に完全に適応し、OpenTelemetryのメトリクスとトレースの両方を一元的に管理できるようになります。

### フェーズ3: Fxアプリケーションへの統合とクリーンアップ

最終フェーズでは、新しいObservabilityモジュールをアプリケーションに組み込み、移行によって不要になった古いコードを削除します。

#### 目的
*   アプリケーション起動時に新しいObservability機能が適切に初期化されるようにする。
*   移行完了後、古いメトリクス関連のコードを完全に削除し、コードベースをクリーンに保つ。

#### 主要な変更点

1.  **Fxアプリケーションの更新**:
    *   アプリケーションのメインFxモジュール（例: `main.go`）を修正し、`pkg/batch/adapter/observability` パッケージをインポートします。
    *   Fxアプリケーションの `fx.Options` に `observability.Module` を追加します。これにより、Observability設定のロードとMeterProvider/TracerProviderの初期化が自動的に行われます。
2.  **古いディレクトリの削除**:
    *   移行が完全に完了し、新しいObservabilityモジュールが機能することを確認した後、古い `pkg/batch/adapter/metrics` ディレクトリを削除します。

#### 影響
*   アプリケーションが新しいObservabilityフレームワークを利用してメトリクスとトレースを収集・エクスポートするようになります。
*   コードベースから冗長な部分が削除され、保守性が向上します。
