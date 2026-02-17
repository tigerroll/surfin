# リソース管理とトランザクション処理の柔軟性向上：段階的実装計画

本ドキュメントは、`docs/strategy/resource_management_and_transaction_flexibility_strategy.md`
に記述された、バッチフレームワークにおけるリソース管理の汎用化とトランザクション処理の柔軟性向上に関する変更を、既存の動作を維持しながら段階的に実装するための手順を定義する。

---

## 目的

*   フレームワークの基盤となるリソース管理を汎用化し、データベース以外のリソース（例: ストレージ）への対応を容易にする。
*   `ItemWriter` のトランザクション処理を汎用化し、データベースに依存しない書き込み処理を可能にする。
*   これらの変更を、既存の機能の動作を損なうことなく、段階的に適用する。

---

## 前提条件

*   現在の `main` ブランチ（または作業対象ブランチ）の全てのテストがパスしていること。
*   新しい機能ブランチを作成し、この計画に沿って作業を進めること。

---

## 進捗管理

| ステップ                                                                                                               | ステータス | 備考 |
| :--------------------------------------------------------------------------------------------------------------------- | :--------- | :--- |
| 1: `pkg/batch/core/tx/tx.go` へのヘルパー関数追加                                                                      | 完了       |      |
| 2: `ItemWriter` インターフェースの変更と、関連する<br/> `ChunkStep` および既存 `ItemWriter` 実装の修正                 | 完了       |      |
| 3: `JobFactory` と関連する JSL 変換ロジックのリソースプロバイダー汎用化<br/>（第一段階：シグネチャ変更とマップの導入） | 完了       |      |
| 4: フレームワークマイグレーションフックの修正                                                                          | 完了       |      |
| 5: `ComponentBuilder` および `buildReaderWriteProcessor` <br/>内部での `ResourceProvider` の利用ロジックの洗練         | 完了       |      |
| 6: `ItemWriter` インターフェースの `GetTargetDBName()` / `GetTableName()` の再検討（オプション）                       | 完了       |      |

## 実装ステップ

### ステップ 1: `pkg/batch/core/tx/tx.go` へのヘルパー関数追加

このステップでは、トランザクションを `context.Context` に格納・取得するためのヘルパー関数を追加する。既存のコードには影響を与えない純粋な追加である。

*   **変更ファイル:** `pkg/batch/core/tx/tx.go`
*   **変更内容:**
    *   `contextKey` 型を定義する。
    *   `TransactionContextKey` 定数を定義する。
    *   `ContextWithTx(ctx context.Context, tx Tx) context.Context` 関数を追加する。
    *   `TxFromContext(ctx context.Context) (Tx, bool)` 関数を追加する。
*   **確認事項:**
    *   追加した関数がコンパイルエラーなく、意図通りに動作することを確認する（簡単な単体テストやデバッグ実行）。
    *   既存の全てのテストが引き続きパスすることを確認する。

### ステップ 2: `ItemWriter` インターフェースの変更と、関連する `ChunkStep` および既存 `ItemWriter` 実装の修正

このステップでは、`ItemWriter` インターフェースから `tx.Tx` 引数を削除し、トランザクションを `context.Context` 経由で渡すように変更する。これは密接に関連する変更であり、同時に適用する必要がある。

*   **変更ファイル:**
    *   `pkg/batch/core/application/port/interfaces.go`
    *   `pkg/batch/engine/step/chunk_step.go`
    *   データベース固有の既存 `ItemWriter` 実装（例: `example/weather/internal/adapter/writer/weather_item_writer.go` など、`ItemWriter` インターフェースを実装している全てのファイル）
*   **変更内容:**
    *   `pkg/batch/core/application/port/interfaces.go`:
        *   `ItemWriter` インターフェースの `Write` メソッドのシグネチャを `Write(ctx context.Context, tx tx.Tx, items []I) error` から `Write(ctx context.Context, items []I) error` に変更する。
    *   `pkg/batch/engine/step/chunk_step.go`:
        *   `ChunkStep` の `Execute` メソッド内で、トランザクションを開始した後、その `tx.Tx` オブジェクトを **ステップ 1 で追加した `tx.ContextWithTx` を使用して `context.Context` に格納し、そのコンテキストを `ItemWriter.Write`
メソッドに渡す** ように変更する。
    *   データベース固有の既存 `ItemWriter` 実装:
        *   `Write` メソッドのシグネチャを `Write(ctx context.Context, tx tx.Tx, items []I) error` から `Write(ctx context.Context, items []I) error` に変更する。
        *   `Write` メソッドの内部で、**ステップ 1 で追加した `tx.TxFromContext` を使用して `context.Context` から `tx.Tx` オブジェクトを取得し、それを使用してデータベース操作を実行する** ように変更する。`tx.TxFromContext` が `false`
を返す場合（トランザクションがコンテキストにない場合）のエラーハンドリングも考慮する。
*   **確認事項:**
    *   全てのコンパイルエラーが解消されていることを確認する。
    *   `ChunkStep` を含むジョブが、以前と同様にデータベースへの書き込みを正しく実行できることを確認する。
    *   既存の全てのテストが引き続きパスすることを確認する。

### ステップ 3: `JobFactory` と関連する JSL 変換ロジックのリソースプロバイダー汎用化（第一段階：シグネチャ変更とマップの導入）

このステップでは、`JobFactory` が `DBConnectionResolver` ではなく、汎用的な `ResourceProvider` のマップを受け取るように変更する。この段階では、`ComponentBuilder` 内部での `ResourceProvider` の具体的な利用ロジックはまだ変更しない。

*   **変更ファイル:**
    *   `pkg/batch/core/config/support/jobfactory.go`
    *   `pkg/batch/core/config/jsl/model.go`
    *   `pkg/batch/core/config/jsl/step_adapter.go`
*   **変更内容:**
    *   `pkg/batch/core/config/support/jobfactory.go`:
        *   `JobFactoryParams` 構造体から `DBResolver coreAdapter.ResourceConnectionResolver` フィールドを削除し、代わりに `AllResourceProviders map[string]coreAdapter.ResourceProvider group:"resourceProviders"` フィールドを追加する。
        *   `JobFactory` 構造体に `resourceProviders map[string]coreAdapter.ResourceProvider` フィールドを追加する。
        *   `NewJobFactory` 関数内で、`JobFactory` の `resourceProviders` フィールドを `p.AllResourceProviders` で初期化する。
        *   `CreateJob` 関数内で、`jsl.ConvertJSLToCoreFlow` の呼び出し時に `f.dbConnectionResolver` の代わりに `f.resourceProviders` を渡すように変更する。
    *   `pkg/batch/core/config/jsl/model.go`:
        *   `ComponentBuilder` 型定義のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更する。
    *   `pkg/batch/core/config/jsl/step_adapter.go`:
        *   `ConvertJSLToCoreFlow` 関数のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更する。
        *   `buildReaderWriteProcessor` 関数のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更する。
        *   `ConvertJSLToCoreFlow` 関数内の `buildReaderWriteProcessor` の呼び出し時に、`dbResolver` の代わりに `resourceProviders` を渡すように変更する。
        *   `buildReaderWriteProcessor` 関数内の `readerBuilder`, `processorBuilder`, `writerBuilder` の呼び出し時に、`dbResolver` の代わりに `resourceProviders` を渡すように変更する。
        *   **重要:** この段階では、`ComponentBuilder` や `buildReaderWriteProcessor` の内部で `resourceProviders` マップを実際に利用するロジックはまだ変更しない。つまり、`dbResolver` を使っていた箇所は、`resourceProviders` マップから
`DBProvider` を取り出し、その `ResolveDBConnection` を呼び出すように変更する。これは、戦略ドキュメントの「4.1. ResourceConnectionResolver と ResourceProvider の役割分担」に直結する。
*   **確認事項:**
    *   全てのコンパイルエラーが解消されていることを確認する。
    *   ジョブのロードと実行が以前と同様に機能することを確認する。
    *   既存の全てのテストが引き続きパスすることを確認する。

### ステップ 4: フレームワークマイグレーションフックの修正

このステップでは、フレームワークマイグレーションを実行するフックが、新しい `AllDBProviders` マップを使用してデータベース接続を取得するように適応させる。

*   **変更ファイル:** `pkg/batch/core/config/bootstrap/module.go`
*   **変更内容:**
    *   `RunFrameworkMigrationsHookParams` 構造体から `DBResolver database.DBConnectionResolver` フィールドを削除する。
    *   `runFrameworkMigrationsHook` 関数内で、データベース接続（`dbConn`）の取得ロジックを、`p.DBResolver.ResolveDBConnection` の呼び出しから、`p.AllDBProviders` マップと `dbConfig.Type` を使用して適切な `database.DBProvider`
を取得し、そこから接続を取得するように変更する。
        *   **考慮事項:** 戦略ドキュメントの「4.2. ResourceProvider.GetConnection と ResourceConnectionResolver.ResolveConnection のシグネチャの違い」および「4.4.
フレームワークマイグレーションフックにおける接続の堅牢性」で述べられているように、`database.DBProvider.GetConnection` が `context.Context`
を引数に取らないため、タイムアウト制御などに注意が必要である。また、`DBConnectionResolver.ResolveDBConnection` が持っていた接続の有効性保証や再確立の責任を `database.DBProvider.GetConnection`
が持つか、あるいは別途ロジックを追加する必要があるかを確認し、必要に応じて対応する。
*   **確認事項:**
    *   全てのコンパイルエラーが解消されていることを確認する。
    *   アプリケーション起動時にフレームワークマイグレーションが正しく実行されることを確認する。
    *   既存の全てのテストが引き続きパスすることを確認する。

### ステップ 5: `ComponentBuilder` および `buildReaderWriteProcessor` 内部での `ResourceProvider` の利用ロジックの洗練

このステップでは、ステップ 3 で導入した `resourceProviders` マップを、より汎用的な方法で利用するように変更する。

*   **変更ファイル:** `pkg/batch/core/config/jsl/step_adapter.go` (主に `buildReaderWriteProcessor` 関数内)
*   **変更内容:**
    *   `buildReaderWriteProcessor` 関数内で、`readerBuilder`, `processorBuilder`, `writerBuilder` を呼び出す際に、`resourceProviders` マップから必要な `ResourceProvider` を取得し、それらをコンポーネントのビルドに渡すように変更する。
    *   例えば、戦略ドキュメントの「4.3. ComponentBuilder を介した ResourceProvider の具体的な利用パターン」で述べられているように、`readerBuilder(cfg, resolver, resourceProviders["database"], readerRef.Properties)` のように、特定の
`ResourceProvider` を明示的に指定して渡すロジックを実装する。
    *   **考慮事項:** `ComponentBuilder` が `map[string]coreAdapter.ResourceProvider` を受け取るようになったので、各コンポーネント（Reader, Processor, Writer）のビルドロジック内で、必要な `ResourceProvider`
をマップから取得し、`GetConnection` を呼び出すように変更する。指定された名前のリソースプロバイダーが見つからない場合のエラーハンドリングも考慮する。
*   **確認事項:**
    *   全てのコンパイルエラーが解消されていることを確認する。
    *   ジョブのロードと実行が以前と同様に機能することを確認する。
    *   既存の全てのテストが引き続きパスすることを確認する。

### ステップ 6: `ItemWriter` インターフェースの `GetTargetDBName()` / `GetTableName()` の再検討（オプション）

このステップは直接的な動作変更ではないが、インターフェースの汎用性を高めるためのクリーンアップである。

*   **変更ファイル:** `pkg/batch/core/application/port/interfaces.go`
*   **変更内容:**
    *   戦略ドキュメントの「4.5. ItemWriter インターフェースの汎用性」で述べられているように、`ItemWriter` インターフェースから `GetTargetDBName()` および `GetTableName()` メソッドを削除するか、より汎用的な名称（例:
`GetTargetResourceName()`, `GetResourcePath()` など）に変更することを検討する。
    *   **考慮事項:** この変更は破壊的変更となるため、既存の `ItemWriter` 実装全てに影響する。非データベースリソースをターゲットとする `ItemWriter` を実装する際に、これらのメソッドの存在が不自然である場合にのみ実施を検討する。
*   **確認事項:**
    *   変更を行った場合、関連する全ての `ItemWriter` 実装と、それらのメソッドを呼び出している箇所を修正し、コンパイルエラーがないことを確認する。
    *   既存の全てのテストが引き続きパスすることを確認する。

---

## 各ステップ完了後の共通確認事項

*   各ステップの完了後には、必ず全ての単体テストおよび統合テストを実行し、既存の機能が壊れていないことを確認すること。
*   特にインターフェースのシグネチャ変更は、呼び出し元と実装の両方を同時に修正する必要があるため、影響範囲を最小限に抑えるように注意すること。
*   変更が意図通りに動作することを確認するために、必要に応じてデバッグログやトレースを活用すること。
