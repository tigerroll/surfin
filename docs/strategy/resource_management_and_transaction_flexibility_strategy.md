# バッチフレームワークにおけるリソース管理とトランザクション処理の柔軟性向上戦略

本ドキュメントは、バッチフレームワークにおけるリソース管理の汎用化とトランザクション処理の柔軟性向上を目的としたコード修正の変更点を記述します。

---

## 1. JobFactory におけるリソースプロバイダー管理の汎用化

**目的:** JobFactory がデータベースに特化した単一のリゾルバーではなく、様々な種類のリソース（データベース、ストレージなど）に対応できる汎用的な `ResourceProvider` のマップを管理するように変更し、拡張性を向上させます。

### 変更ファイル:
* `pkg/batch/core/config/support/jobfactory.go`
* `pkg/batch/core/config/jsl/model.go`
* `pkg/batch/core/config/jsl/step_adapter.go`

### 変更詳細:
* **`pkg/batch/core/config/support/jobfactory.go`**:
    * `JobFactoryParams` 構造体から `DBResolver coreAdapter.ResourceConnectionResolver` フィールドを削除し、代わりに `AllResourceProviders map[string]coreAdapter.ResourceProvider group:"resourceProviders"` フィールドを追加します。
    * `JobFactory` 構造体に `resourceProviders map[string]coreAdapter.ResourceProvider` フィールドを追加します。
    * `NewJobFactory` 関数内で、`JobFactory` の `resourceProviders` フィールドを `p.AllResourceProviders` で初期化します。
    * `CreateJob` 関数内で、`jsl.ConvertJSLToCoreFlow` の呼び出し時に `f.dbConnectionResolver` の代わりに `f.resourceProviders` を渡すように変更します。
* **`pkg/batch/core/config/jsl/model.go`**:
    * `ComponentBuilder` 型定義のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更します。
* **`pkg/batch/core/config/jsl/step_adapter.go`**:
    * `ConvertJSLToCoreFlow` 関数のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更します。
    * `ConvertJSLToCoreFlow` 関数内の `buildReaderWriteProcessor` の呼び出し時に、`dbResolver` の代わりに `resourceProviders` を渡すように変更します。
    * `buildReaderWriteProcessor` 関数のシグネチャを、`dbResolver coreAdapter.ResourceConnectionResolver` パラメータから `resourceProviders map[string]coreAdapter.ResourceProvider` パラメータに変更します。
    * `buildReaderWriteProcessor` 関数内の `readerBuilder`, `processorBuilder`, `writerBuilder` の呼び出し時に、`dbResolver` の代わりに `resourceProviders` を渡すように変更します。

---

## 2. フレームワークマイグレーションフックの修正

**目的:** `JobFactoryParams` から `DBResolver` を削除する変更に伴い、フレームワークマイグレーションを実行するフックが、新しい `AllDBProviders` マップを使用してデータベース接続を取得するように適応させます。

### 変更ファイル:
* `pkg/batch/core/config/bootstrap/module.go`

### 変更詳細:
* **`pkg/batch/core/config/bootstrap/module.go`**:
    * `RunFrameworkMigrationsHookParams` 構造体から `DBResolver database.DBConnectionResolver` フィールドを削除します。
    * `runFrameworkMigrationsHook` 関数内で、データベース接続（`dbConn`）の取得ロジックを、`p.DBResolver.ResolveDBConnection` の呼び出しから、`p.AllDBProviders` マップと `dbConfig.Type` を使用して適切な `database.DBProvider` を取得し、そこから接続を取得するように変更します。

---

## 3. ItemWriter のトランザクション処理の汎用化

**目的:** `ItemWriter` インターフェースをデータベーストランザクションに直接依存しないように汎用化し、ストレージなどの非データベース書き込み処理にも対応できるようにします。トランザクションは `context.Context` を介して渡されるようにします。

### 変更ファイル:
* `pkg/batch/core/application/port/interfaces.go`
* `pkg/batch/core/tx/tx.go`
* `pkg/batch/engine/step/chunk_step.go` （概念的な変更）
* データベース固有の `ItemWriter` 実装（概念的な変更）

### 変更詳細:
* **`pkg/batch/core/application/port/interfaces.go`**:
    * `ItemWriter` インターフェースの `Write` メソッドのシグネチャを `Write(ctx context.Context, tx tx.Tx, items []I) error` から `Write(ctx context.Context, items []I) error` に変更し、`tx tx.Tx` 引数を削除します。
* **`pkg/batch/core/tx/tx.go`**:
    * `contextKey` 型と `TransactionContextKey` 定数を追加します。
    * `ContextWithTx(ctx context.Context, tx Tx) context.Context` ヘルパー関数を追加します。
    * `TxFromContext(ctx context.Context) (Tx, bool)` ヘルパー関数を追加します。
* **`pkg/batch/engine/step/chunk_step.go` （概念的な変更）**:
    * `ChunkStep` の `Execute` メソッド内で、トランザクションを開始した後、その `tx.Tx` オブジェクトを `tx.ContextWithTx` を使用して `context.Context` に格納し、そのコンテキストを `ItemWriter.Write` メソッドに渡すように変更します。
* **データベース固有の `ItemWriter` 実装（概念的な変更）**:
    * データベースに書き込む `ItemWriter` の実装は、`Write` メソッド内で `tx.TxFromContext` を使用して `context.Context` から `tx.Tx` オブジェクトを取得し、それを使用してデータベース操作を実行するように変更します。

---

## 4. 実装上の追加考慮事項

上記変更を実際に実装するにあたり、以下の観点も考慮する必要があります。

### 4.1. ResourceConnectionResolver と ResourceProvider の役割分担

*   `JobFactoryParams` および `ComponentBuilder` のシグネチャ変更により、`DBConnectionResolver` が `ResourceProvider` のマップに置き換えられます。
*   `coreAdapter.ResourceConnectionResolver` は `ResolveConnection` メソッドを通じて接続の有効性保証や再確立の責任を持つと定義されています。
*   `ResourceProvider` の `GetConnection` メソッドが、この `ResolveConnection` と同等の機能（接続の有効性チェック、必要に応じた再確立）を提供するか、あるいはその機能が別の方法で提供されるかについて、実装時に明確な設計判断が必要です。

### 4.2. ResourceProvider.GetConnection と ResourceConnectionResolver.ResolveConnection のシグネチャの違い

*   `coreAdapter.ResourceConnectionResolver` の `ResolveConnection` メソッドは `context.Context` を引数に取りますが、`coreAdapter.ResourceProvider` の `GetConnection` メソッドは `context.Context` を取りません。
*   `context.Context` はタイムアウトやキャンセルシグナルを伝播するために重要です。`GetConnection` が `Context` を受け取らない場合、接続の確立や取得に時間がかかった際のタイムアウト制御などが難しくなる可能性があります。このシグネチャの違いが、接続のライフサイクル管理やエラーハンドリングに与える影響について、実装時に考慮が必要です。

### 4.3. ComponentBuilder を介した ResourceProvider の具体的な利用パターン

*   `ComponentBuilder` が `map[string]coreAdapter.ResourceProvider` を受け取るようになりますが、各 `ItemReader`、`ItemProcessor`、`ItemWriter` の実装が、このマップからどのようにして必要な `ResourceProvider` を特定し、そこから `ResourceConnection` を取得するのか、その具体的な実装パターンがドキュメントには明示されていません。
*   例えば、`ItemWriter` が特定のデータベース接続を必要とする場合、`resourceProviders["database"]` のようにマップから `database.DBProvider` を取得し、その `GetConnection` メソッドを呼び出すといった具体的なロジックが必要になります。この取得ロジックと、それに伴うエラーハンドリング（指定された名前のリソースプロバイダーが見つからない場合など）の考慮が必要です。

### 4.4. フレームワークマイグレーションフックにおける接続の堅牢性

*   `runFrameworkMigrationsHook` 関数内で、データベース接続の取得方法が `DBConnectionResolver.ResolveDBConnection` から `database.DBProvider.GetConnection` に変更されます。
*   `DBConnectionResolver.ResolveDBConnection` は接続の有効性保証と再確立の責任を持つとされていますが、`database.DBProvider.GetConnection` が同様の堅牢性を提供するかどうかを確認する必要があります。
*   もし `DBProvider.GetConnection` が単に既存の接続を返すだけであれば、マイグレーション実行時の接続の安定性を確保するために、追加の有効性チェックや再接続ロジックが必要となる可能性があります。

### 4.5. ItemWriter インターフェースの汎用性

*   `ItemWriter` インターフェースの `Write` メソッドはトランザクションから独立しますが、`GetTargetDBName()` および `GetTableName()` メソッドは依然としてデータベース固有の情報を返します。
*   ストレージなどの非データベースリソースをターゲットとする `ItemWriter` を実装する場合、これらのメソッドの存在がインターフェースの汎用性という目的と矛盾する可能性があります。
*   将来的な拡張性やインターフェースの整合性を考慮し、これらのメソッドの必要性、あるいはより汎用的な名称（例: `GetTargetResourceName()`, `GetResourcePath()` など）への変更を検討する余地があります。

### 4.6. ResourceProvider の登録と設定

*   `JobFactory` が `map[string]coreAdapter.ResourceProvider` を受け取るようになりますが、これらの `ResourceProvider` インスタンスがどのように構築され、フレームワークに登録されるのか（例: Fx の `fx.Provide` と `fx.Group` を使った登録メカニズム、それぞれの `ResourceProvider` が持つべき設定構造など）について、具体的な実装方法の検討が必要です。

### 4.7. ResourceConnection のライフサイクル管理

*   `ResourceProvider` から取得した `ResourceConnection` の `Close()` メソッドを誰が、いつ呼び出すのか、その責任範囲を明確にする必要があります。特に、`ItemReader` や `ItemWriter` が `Open()` で接続を取得し、`Close()` で解放するパターンが想定されますが、`ChunkStep` のような上位のコンポーネントが接続のライフサイクルを管理する必要があるか、あるいは `ResourceProvider` 自体が接続プールを管理し、明示的な `Close()`
が不要な場合があるかなど、詳細な設計が必要です。

### 4.8. 非データベースリソースにおけるトランザクション類似の概念

*   `ItemWriter` が汎用化され、`tx.Tx` を直接受け取らなくなるものの、S3 などの非データベースリソースに対する書き込みにおいて、データベースのトランザクション（ACID 特性）に相当する「一貫性」や「アトミック性」をどのように保証するのか、その戦略を検討する必要があります。`tx.Tx`
インターフェースが依然としてデータベース操作に特化しているため、異なるリソースタイプ間での整合性確保には、より上位の調整メカニズムや、各リソースタイプに応じた異なるアプローチが必要となる可能性があります。
