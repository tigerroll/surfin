<p align="center">
  <img src="docs/images/surfin-logo.png" alt="Surfin Logo" width="150"/>
</p>
<center>
  [English](README.md) | [日本語](README.ja.md)
</center>

# 🌊 Surfin - Batch framework

[![GoDoc](https://pkg.go.dev/badge/github.com/tigerroll/surfin.svg)](https://pkg.go.dev/badge/github.com/tigerroll/surfin)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tigerroll/surfin)](https://goreportcard.com/report/github.com/tigerroll/surfin)

**JSR-352 にインスパイアされた Go 言語による軽量バッチフレームワーク**

**Surfin - Batch framework** は、堅牢性、スケーラビリティ、および運用容易性を最優先に開発を進めています。宣言的なジョブ定義 (JSL) とクリーンなアーキテクチャにより、複雑なデータ処理タスクを効率的かつ信頼性の高い方法で実行します。

---

## 🌟 Why Choose Surfin?

*  **🚀 Go Performance:** Go 言語のネイティブな並行処理とパフォーマンスを最大限に活用
*  **🏗️ Convention over Configuration (CoC):** 規約（CoC）による統治でボイラープレートを排除し、最小限の実装で堅牢性を担保
*  **✨ Observable:** Prometheus/OpenTelemetry をコアに統合し、規約に従うだけで、分散トレーシングとメトリクス収集がを実現
*  **🛠️ Flexible:** 動的DBルーティングとUber Fx (DI) により、複雑なインフラとカスタム要件に柔軟に対応
*  **🔒 Robust:** 永続メタデータ、楽観的ロック、高粒度なエラー処理（リトライ/スキップ）によるフォールトトレランス機能
*  **🎯 JSR-352 Compliant:** 業界標準のバッチ処理モデル（JSR-352）にインスパイアされた実装
*  **📈 Scalable:** Remote Worker 連携による分散実行と Partition の抽象化により、大規模なデータセットに対応

---

## 🛠️  Key Features

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

### ✨ 運用容易性とオブザーバビリティ
*   **OpenTelemetry/Prometheus 統合:** フレームワークのコアに分散トレーシング（Job/Step Span 自動生成）とメトリクス収集（MetricRecorder）を統合。短命なバッチでも確実な監視を保証し、Remote Scheduler との連携基盤を提供。
*   **高粒度なエラーポリシー:** エラーの特性（Transient, Skippable）に基づく、宣言的なリトライ/スキップ戦略の設定をサポート。

### 🌐 スケーラビリティとインフラストラクチャ
*   **動的DI**: Uber Fx による依存性注入を採用し、コンポーネントの動的な構築とライフサイクル管理を実現。
*   **動的データソースルーティング**: 実行コンテキストに基づいて、複数のデータソース（Postgres, MySQL など）を動的に切り替える機構をサポート。
*   **分散実行抽象化**: `Partitioner` と `StepExecutor` の抽象化により、ローカル実行からリモートワーカー（Kubernetes Job など）への実行委譲をサポート。

---

## 🚀 Getting Started

Surfin を使用した最小構成のアプリケーションを構築するには、以下のガイドを参照してください。

👉 **[tutorial - "Hello, Wold!"](docs/tutorial/hello-world.md)**

## 📚 Documentation & Usage

より詳細な機能、設定、および複雑なジョブフローの構築については、以下のドキュメントを参照してください。

*   **[アーキテクチャと設計原則](docs/architecture)**: フレームワークのアーキテクチャ、設計原則、およびプロジェクト構造に関する詳細情報を参照してください。
*   **[Surfin バッチアプリケーションの作成ガイド](docs/guide/usage.md)**: JSL、カスタムコンポーネント、リスナー、フロー制御の完全なガイド。
*   **[実装ロードマップ](docs/strategy/adapter_and_component_roadmap.md)**: フレームワークの設計目標と達成状況。

---

## 🆘 Support

ご質問や問題が発生した場合は、GitHub Issues をご利用ください。

*   **GitHub Issues**: [Report bugs or request features](https://github.com/tigerroll/surfin/issues)
----
