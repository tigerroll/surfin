# 2. レイヤー構造とパッケージ対応

Surfin は以下のレイヤーで構成されています。

| レイヤー | パッケージ | 責務 |
| :--- | :--- | :--- |
| **Core** | `pkg/batch/core/` | フレームワークの核となるインターフェース、モデル、実行ロジック。インフラ層に依存しない。 |
| **Engine** | `pkg/batch/engine/` | バッチ処理の実行エンジン（`StepExecutor`）や、具体的なステップ実装（`ChunkStep`, `TaskletStep`）。 |
| **Infrastructure** | `pkg/batch/infrastructure/` | `Core` インターフェースの具体的な実装（GORM を利用したリポジトリ、DB/Tx マネージャーなど）。 |
| **Adapter** | `pkg/batch/adapter/` | 外部システム（データベース、メッセージングなど）との接続を抽象化する具体的な実装。 |
| **Component** | `pkg/batch/component/` | 再利用可能なバッチコンポーネント（`NoOpPartitioner` など）。 |
| **Listener** | `pkg/batch/listener/` | ライフサイクルイベントを処理するリスナーの実装。 |
| **Support** | `pkg/batch/support/` | 汎用ユーティリティ（ロギング、例外処理、シリアライズなど）。 |
