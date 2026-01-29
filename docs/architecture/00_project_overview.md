# 0. プロジェクト概要と主要機能

## Overview

**Surfin - Batch framework** は、責務を明確に分離したレイヤードアーキテクチャを採用しています。<br/>堅牢で保守性の高いシステムを実現するために、 **責務の分離 (Separation of Concerns)** 、 **依存性逆転の原則 (DIP)** 、 **拡張性(Extensibility)** 、そして **トランザクション境界の明確化** といった主要な設計原則に遵っています。

⚙️ フレームワークの層構造に関する詳細は、[2. フレームワークの層構造と実行フロー](02_layer_and_flow.md) を参照してください。

このアーキテクチャは、**依存性逆転の原則 (DIP)** を意識しています。<br/> `Core` レイヤーはフレームワークの「心臓部」であり、`port` パッケージで定義された抽象的なインターフェース群を含みます。<br/>`Infrastructure` や `Adapter`
レイヤーは、これらの `Core` インターフェースの具体的な実装を提供します。<br/>これにより、`Core` は特定のデータベースや外部システムの実装に依存せず、移植性とテスト容易性を実現しています。<br/>`Engine` レイヤーは、`Core`
で定義された実行ロジック（`Step`, `Job` など）を実際に動かすための具体的な実装を提供します。

---

## 🚀 主要機能

### ⚙️ コア機能とフロー制御

*   **宣言的ジョブ定義 (JSL)**: YAML ベースの JSL により、ジョブ、ステップ、コンポーネント、トランジションを宣言的に定義。
*   **チャンク/タスクレットモデル**: `ItemReader`, `ItemProcessor`, `ItemWriter` によるチャンク指向処理と、単一タスク実行のための `Tasklet` をサポート。
*   **高度なフロー制御**: 条件分岐 (`Decision`)、並列実行 (`Split`)、および柔軟な遷移ルール (`Transition`) による複雑なジョブフローの構築。
*   **リスナーモデル**: Job, Step, Chunk, Item の各ライフサイクルイベントにカスタムロジックを挿入可能。

### 🛡️ 堅牢性とメタデータ管理

*   **再起動可能性**: `JobRepository` と `ExecutionContext` により、失敗したジョブを中断点から正確に再開可能。
*   **フォールトトレランス**: アイテムレベルの**リトライ**および**スキップ**ポリシーをサポート。書き込みエラー時にはチャンク分割によるスキップロジックを自動適用。
*   **楽観的ロック**: メタデータリポジトリ層に楽観的ロックを実装し、分散環境でのデータ整合性を保証。
*   **機密情報マスキング**: `JobParameters` の永続化およびログ出力時に、機密情報を自動的にマスキング。

### ✨ 運用成熟度とオブザーバビリティ

*   **OpenTelemetry/Prometheus 統合:** フレームワークのコアに分散トレーシング（Job/Step Span 自動生成）とメトリクス収集（MetricRecorder）を統合。短命なバッチでも確実な監視を保証し、Remote Partition との連携基盤を提供。
*   **高粒度なエラーポリシー:** エラーの特性（Transient, Skippable）に基づく、宣言的なリトライ/スキップ戦略の設定をサポート。

### 🌐 スケーラビリティとインフラストラクチャ

*   **動的DI**: Uber Fx による依存性注入を採用し、コンポーネントの動的な構築とライフサイクル管理を実現。
*   **動的データソースルーティング**: 実行コンテキストに基づいて、複数のデータソース（Postgres, MySQL など）を動的に切り替える機構をサポート。
*   **分散実行抽象化**: `Partitioner` と `StepExecutor` の抽象化により、ローカル実行からリモートワーカー（Kubernetes Job など）への実行委譲をサポート。

---

## 🏗 Architecture

**Surfin - Batch framework** は、責務を明確に分離したレイヤードアーキテクチャを採用しています。<br/>堅牢で保守性の高いシステムを実現するために、 **責務の分離 (Separation of Concerns)** 、 **依存性逆転の原則 (DIP)** 、 **拡張性(Extensibility)** 、そして **トランザクション境界の明確化** といった主要な設計原則に遵っています。

⚙ 以下の明確に分離されたレイヤーで構成されています。

| レイヤー | パッケージ | 責務 |
| :--- | :--- | :--- |
| **Adapter** | `pkg/batch/adapter/` | 外部システム（データベース、メッセージングなど）との接続を抽象化する**具体的な実装**を提供するアダプター層。`Core` で定義されたインターフェースを実装します。 |
| **Component** | `pkg/batch/component/` | フレームワークが提供する、再利用可能なバッチコンポーネント（`ItemReader`, `ItemProcessor`, `Tasklet` など）の実装。 |
| **Core** | `pkg/batch/core/` | フレームワークの核となるインターフェース、モデル、実行ロジック（`JobRunner`, `StepExecutor`）。<br/>インフラストラクチャ層に依存しない。 |
| **Engine** | `pkg/batch/engine/` | バッチ処理の実行エンジン（`StepExecutor`）や、`ChunkStep`, `TaskletStep`, `PartitionStep` といった具体的なステップの実装。`Core` で定義された抽象を具現化します。 |
| **Infrastructure** | `pkg/batch/infrastructure/` | 外部システムとの接続（DB、トランザクション、リポジトリ）の具体的な実装。`Core` で定義されたインターフェースを実装します。 |
| **Listener** | `pkg/batch/listener/` | ジョブ/ステップのライフサイクルイベントを処理するリスナーの具体的な実装。 |
| **Support** | `pkg/batch/support/` | フレームワーク全体で利用される汎用的なユーティリティ（ロギング、例外処理、シリアライズなど）。 |
| **Test** | `pkg/batch/test/` | フレームワークのテストユーティリティやモック。 |
| **Application** | `example/weather/internal/` | アプリケーション固有のビジネスロジック（`ItemReader`, `ItemProcessor`, `ItemWriter` の実装）や設定。 |

このアーキテクチャは、**依存性逆転の原則 (DIP)** を意識しています。<br/> `Core` レイヤーはフレームワークの「心臓部」であり、`port` パッケージで定義された抽象的なインターフェース群を含みます。<br/>`Infrastructure` や `Adapter`
レイヤーは、これらの `Core` インターフェースの具体的な実装を提供します。<br/>これにより、`Core` は特定のデータベースや外部システムの実装に依存せず、移植性とテスト容易性を実現しています。<br/>`Engine` レイヤーは、`Core`
で定義された実行ロジック（`Step`, `Job` など）を実際に動かすための具体的な実装を提供します。

## 📁 Project Structure

主要なディレクトリ構造は以下の通りです。

```
├── pkg/batch/              # Surfin Batch Framework Core
│   ├── adapter/            # 外部システム（データベース、メッセージングなど）との接続を抽象化する具体的な実装。
│   ├── component/          # フレームワークが提供する再利用可能なバッチコンポーネント（例: `NoOpPartitioner`）。
│   ├── core/               # フレームワークの核となるインターフェース、モデル、実行ロジック。インフラストラクチャ層に依存しない。
│   │   ├── adapter/        # データベース接続やトランザクションマネージャーなど、外部システムとの接続を抽象化する**インターフェース**。
│   │   ├── application/    # アプリケーション層のインターフェース (`port` パッケージ)。
│   │   ├── config/         # フレームワークの設定定義 (`Config` 構造体)、JSLモデル、および設定ロードロジック。
│   │   ├── domain/         # ドメインモデル (`model` パッケージ) とリポジトリインターフェース (`repository` パッケージ)。
│   │   ├── metrics/        # メトリクス収集とトレーシングのインターフェースと抽象化。
│   │   └── tx/             # トランザクション管理のインターフェースと抽象化。
│   ├── engine/             # バッチ処理の実行エンジン（`StepExecutor`）と、`ChunkStep`, `TaskletStep`, `PartitionStep` などの具体的なステップ実装。
│   ├── infrastructure/     # Core インターフェースの具体的な実装（GORM を利用したリポジトリ、DB/Tx マネージャーなど）
│   │   ├── repository/     # `JobRepository` などの永続化層の具体的な実装（例: GORM を利用）。
│   ├── listener/           # ジョブ/ステップのライフサイクルイベントを処理するリスナーの具体的な実装。
│   ├── support/            # フレームワーク全体で利用される汎用ユーティリティ（ロギング、例外処理、シリアライズ、JobFactory など）。
│   └── test/               # フレームワークのテストユーティリティやモック。
├── example/weather/        # Example Application (Weather Data Processing)
│   ├── cmd/weather/        # Application entry point (main.go, resources)
│   └── internal/           # Application specific logic (steps, domain)
└── docs/                   # Documentation and Guides
```

---
