# アダプター層リファクタリング フェーズ2: DIコンテナの調整とCore層の抽象化

このドキュメントは、`docs/strategy/adapter_refactoring_strategy.md` で定義されたアダプター層リファクタリングの「フェーズ2: DIコンテナの調整」を、動作を維持しながら段階的に進めるための具体的なタスクリストです。

## 目的

*   DIコンテナが、新しいインターフェース定義に基づいて依存性を正しく解決できるようにする。
*   具体的なデータベース実装が、汎用インターフェースとしても提供されることを確認する。
*   Core層のライブラリパッケージから、特定のアダプター（`pkg/batch/adapter/database` など）への直接的な依存を排除し、`pkg/batch/core/adapter` で定義された汎用インターフェースに依存させる。

## タスクリスト

### タスク 2.1: プロバイダーの調整の確認

**目的**: `pkg/batch/adapter/database/gorm/module.go` および関連するプロバイダー実装が、`database.DBConnection` や `database.DBProvider` といった具体的なインターフェースだけでなく、`core/adapter.ResourceConnection` や `core/adapter.ResourceProvider`
といった汎用インターフェースとしても解決されるように、`go.uber.org/fx.As` を使用して調整されていることを確認します。

**現在の状況**:
このタスクは、前回の「フェーズ1: 汎用インターフェースの定義と既存コードの分離」における「タスク 1.3: DIコンテナのプロバイダー修正」として既に完了しています。

**変更内容**:
*   **変更は不要です。** 既に以下のファイルで `fx.As(new(database.DBProvider), new(coreAdapter.ResourceProvider))` が適用されています。
    *   `pkg/batch/adapter/database/gorm/mysql/module.go`
    *   `pkg/batch/adapter/database/gorm/postgres/module.go`
    *   `pkg/batch/adapter/database/gorm/sqlite/module.go`

**確認事項**:
*   プロジェクト全体で `go build ./...` が成功すること。
*   アプリケーションの主要な機能（例: ジョブの実行、データベース操作）が正常に動作すること。
*   既存の単体テスト、統合テストが引き続き成功すること。

### タスク 2.2: Core層のインターフェースの抽象化と直接インポートの排除

**目的**: `pkg/batch/core` 配下の主要なパッケージ（`domain/repository`, `engine/step`, `config/jsl`, `config/support`, `tx` など）から `pkg/batch/adapter/database` パッケージへの直接的なインポートを排除し、`pkg/batch/core/adapter`
で定義された汎用インターフェースに依存させる。

**変更内容**:
*   `pkg/batch/core` 配下のファイルにおいて、`github.com/tigerroll/surfin/pkg/batch/adapter/database` のインポートを削除する。
*   `database.DBConnection` 型を使用している箇所を `core/adapter.ResourceConnection` に変更する。
*   `database.DBConnectionResolver` 型を使用している箇所を `core/adapter.ResourceConnectionResolver` に変更する。
*   `pkg/batch/core/tx/tx.go` 内の `TransactionManagerFactory` インターフェースの `NewTransactionManager` メソッドの引数 `dbConn` の型を `database.DBConnection` から `core/adapter.ResourceConnection` に変更する。
*   変更に伴い、具体的なデータベース操作（クエリ実行、テーブル存在チェックなど）が必要な箇所では、`core/adapter.ResourceConnection` を `database.DBConnection`
に**型アサーション**して、具体的なメソッドを呼び出す。このアプローチは、このフェーズにおける「簡易的な方法」として採用します。

**影響範囲**:
*   `pkg/batch/core/domain/repository/repository.go`
*   `pkg/batch/infrastructure/repository/sql/repository.go`
*   `pkg/batch/engine/step/item/chunk_step.go`
*   `pkg/batch/engine/step/factory/factory.go`
*   `pkg/batch/core/config/jsl/model.go`
*   `pkg/batch/core/config/jsl/step_adapter.go`
*   `pkg/batch/core/config/support/jobfactory.go`
*   `pkg/batch/core/tx/tx.go`
*   `example/weather/internal/step/writer/module.go`
*   `example/weather/internal/step/processor/module.go`
*   `example/weather/internal/step/reader/module.go`
*   `pkg/batch/component/item/module.go`
*   `pkg/batch/component/item/execution_context_item_writer.go`
*   `pkg/batch/component/tasklet/generic/module.go`
*   `pkg/batch/component/tasklet/migration/module.go`
*   `pkg/batch/component/tasklet/migration/tasklet.go`
*   `pkg/batch/component/tasklet/migration/migrator.go`
*   `pkg/batch/component/tasklet/migration/migrator_provider.go`
*   `example/weather/internal/app/module.go`

**確認事項**:
*   プロジェクト全体で `go build ./...` が成功すること。
*   アプリケーションの主要な機能（例: ジョブの実行、データベース操作）がこれまで通り正常に動作すること。
*   既存の単体テスト、統合テストが引き続き成功すること。
*   `pkg/batch/core` 配下のパッケージが `pkg/batch/adapter/database` を直接インポートしていないことを確認する。

---

**フェーズ2の完了**:
上記のタスクがすべて完了し、アプリケーションの動作が維持されていることを確認できれば、「フェーズ2: DIコンテナの調整とCore層の抽象化」は完了です。

次のステップとして、「フェーズ3: 新しいリソースタイプのアダプター追加 (例: Storage)」に進むことを推奨します。
