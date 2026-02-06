# アダプター層リファクタリング フェーズ1: 実装タスクリスト

このドキュメントは、`docs/strategy/adapter_refactoring_strategy.md` で定義されたアダプター層リファクタリングの「フェーズ1: 汎用インターフェースの定義と既存コードの分離」を、動作を維持しながら段階的に進めるための具体的なタスクリストです。

## 目的

*   Core層から特定の技術（データベース）への直接的な依存を排除し、汎用的な抽象インターフェースに依存させる。
*   既存のデータベース関連インターフェースを、より具体的なアダプターパッケージへ移動・再定義する。
*   各タスク完了後もアプリケーションがコンパイル可能であり、基本的な動作が維持されることを目指す。

## タスクリスト

### タスク 1.1: 汎用アダプターインターフェースの定義

**目的**: アプリケーションのCore層が依存する、最も抽象的なリソース操作の共通規格を定義します。

**変更内容**:
*   `pkg/batch/core/adapter/interfaces.go` を新規作成します。
*   以下のインターフェースを `pkg/batch/core/adapter/interfaces.go` に定義します。
    *   `ResourceConnection`
    *   `ResourceProvider`
    *   `ResourceConnectionResolver`
*   **注意**: この時点では、既存の `pkg/batch/core/adapter/adapter.go` は削除せず、コンパイルエラーが発生しないようにします。

**影響範囲**:
*   新規ファイル作成のみ。既存コードへの直接的な影響はありません。

**確認事項**:
*   `pkg/batch/core/adapter/interfaces.go` が正しく作成され、インターフェースが定義されていること。
*   プロジェクト全体で `go build ./...` が成功すること。

### タスク 1.2: データベース固有インターフェースの移動と `adapter.go` の削除

**目的**: データベースに特化したインターフェースを専用パッケージに移動し、Core層からデータベース固有の定義を分離します。

**変更内容**:
*   `pkg/batch/adapter/database/interfaces.go` を新規作成します。
*   `pkg/batch/core/adapter/adapter.go` から以下のインターフェース定義を `pkg/batch/adapter/database/interfaces.go` に移動します。
    *   `DBExecutor`
    *   `DBConnection`
    *   `DBProvider`
    *   `DBConnectionResolver`
*   移動した `DBConnection` インターフェースは、`core/adapter.ResourceConnection` と `DBExecutor` を埋め込むように修正します。
*   移動した `DBProvider` インターフェースは、`core/adapter.ResourceProvider` を埋め込むように修正します。
*   移動した `DBConnectionResolver` インターフェースは、`core/adapter.ResourceConnectionResolver` を埋め込むように修正します。
*   `pkg/batch/core/adapter/adapter.go` を削除します。
*   この変更により発生するコンパイルエラーを修正します。具体的には、以下の箇所でインポートパスと型名を修正します。
    *   `pkg/batch/core/domain/repository` 配下の各リポジトリインターフェース（例: `JobExecution`, `JobInstance`, `StepExecution`）が `adapter.DBConnection` を使用している場合、`database.DBConnection` に変更します。
    *   `pkg/batch/adapter/database/gorm` パッケージ内のファイル（例: `module.go`, `provider.go` など）で、旧 `adapter` パッケージの型を参照している箇所を `database` パッケージの型に修正します。
    *   `DBProviderGroup` 定数が参照されている箇所があれば、適切な場所（例: `pkg/batch/adapter/database/constants.go` など、または不要であれば削除）に移動するか、参照を修正します。

**影響範囲**:
*   `pkg/batch/core/adapter/adapter.go` の削除。
*   `pkg/batch/adapter/database/interfaces.go` の新規作成。
*   `pkg/batch/core/domain/repository` および `pkg/batch/adapter/database/gorm` パッケージ内の既存コードの修正。

**確認事項**:
*   `pkg/batch/core/adapter/adapter.go` が削除されていること。
*   `pkg/batch/adapter/database/interfaces.go` にデータベース関連のインターフェースが正しく移動・修正されていること。
*   プロジェクト全体で `go build ./...` が成功すること。
*   既存の単体テスト、統合テストが引き続き成功すること。

### タスク 1.3: DIコンテナのプロバイダー修正

**目的**: DIコンテナが、新しいインターフェース定義に基づいて依存性を正しく解決できるようにします。

**変更内容**:
*   `pkg/batch/adapter/database/gorm/module.go` および関連するプロバイダー実装（例: `pkg/batch/adapter/database/gorm/provider.go` など）を修正します。
*   `fx.Provide` で提供される具体的な実装が、`database.DBConnection` や `database.DBProvider` といった具体的なインターフェースだけでなく、`core/adapter.ResourceConnection` や `core/adapter.ResourceProvider`
といった汎用インターフェースとしても解決されるように、`go.uber.org/fx.As` を使用してプロバイダーを調整します。

**影響範囲**:
*   `pkg/batch/adapter/database/gorm` パッケージ内のDIコンテナ関連コード。

**確認事項**:
*   DIコンテナがアプリケーション起動時にエラーなく依存性を解決できること。
*   アプリケーションの主要な機能（例: ジョブの実行、データベース操作）が正常に動作すること。
*   プロジェクト全体で `go build ./...` が成功すること。
*   既存の単体テスト、統合テストが引き続き成功すること。
