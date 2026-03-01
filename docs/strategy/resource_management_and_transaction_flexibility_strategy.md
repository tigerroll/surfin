# バッチフレームワークにおけるリソース管理とトランザクション処理の柔軟性向上戦略

本ドキュメントは、バッチフレームワークにおけるリソース管理の汎用化とトランザクション処理の柔軟性向上を目的としたコード修正の変更点を記述します。

---

## 1. JobFactory におけるリソースプロバイダー管理の汎用化

**目的:** JobFactory がデータベースに特化した単一のリゾルバーではなく、様々な種類のリソース（データベース、ストレージなど）に対応できる汎用的な `ResourceProvider` のマップを管理するように変更し、拡張性を向上させました。

### 実装済みファイル:
* `pkg/batch/core/config/support/jobfactory.go`
* `pkg/batch/core/config/jsl/model.go`
* `pkg/batch/core/config/jsl/step_adapter.go`

### 実装詳細:
* **`pkg/batch/core/config/support/jobfactory.go`**:
    * `JobFactoryParams` 構造体は `AllResourceProviders map[string]coreAdapter.ResourceProvider group:"resourceProviders"` フィールドを持つように変更されました。
    * `JobFactory` 構造体は `resourceProviders map[string]coreAdapter.ResourceProvider` フィールドを持つように変更されました。
    * `NewJobFactory` 関数内で、`JobFactory` の `resourceProviders` フィールドは `p.AllResourceProviders` で初期化されます。
    * `CreateJob` 関数内で、`jsl.ConvertJSLToCoreFlow` の呼び出し時に `f.resourceProviders` が渡されるように変更されました。
* **`pkg/batch/core/config/jsl/model.go`**:
    * `ComponentBuilder` 型定義のシグネチャは、`resourceProviders map[string]coreAdapter.ResourceProvider` パラメータを受け取るように変更されました。
* **`pkg/batch/core/config/jsl/step_adapter.go`**:
    * `ConvertJSLToCoreFlow` 関数のシグネチャは、`resourceProviders map[string]coreAdapter.ResourceProvider` パラメータを受け取るように変更されました。
    * `ConvertJSLToCoreFlow` 関数内の `buildReaderWriteProcessor` の呼び出し時に、`resourceProviders` が渡されるように変更されました。
    * `buildReaderWriteProcessor` 関数のシグネチャは、`resourceProviders map[string]coreAdapter.ResourceProvider` パラメータを受け取るように変更されました。
    * `buildReaderWriteProcessor` 関数内の `readerBuilder`, `processorBuilder`, `writerBuilder` の呼び出し時に、`resourceProviders` が渡されるように変更されました。

---

## 2. フレームワークマイグレーションフックの修正

**目的:** `JobFactoryParams` から `DBResolver` を削除する変更に伴い、フレームワークマイグレーションを実行するフックが、新しい `AllDBProviders` マップを使用してデータベース接続を取得するように適応されました。

### 実装済みファイル:
* `pkg/batch/core/config/bootstrap/module.go`

### 実装詳細:
* **`pkg/batch/core/config/bootstrap/module.go`**:
    * `RunFrameworkMigrationsHookParams` 構造体から `DBResolver database.DBConnectionResolver` フィールドは削除されました。
    * `runFrameworkMigrationsHook` 関数内で、データベース接続（`dbConn`）の取得ロジックは、`p.AllDBProviders` マップと `dbConfig.Type` を使用して適切な `database.DBProvider` を取得し、そこから接続を取得するように変更されました。

---

## 3. ItemWriter のトランザクション処理の汎用化

**目的:** `ItemWriter` インターフェースをデータベーストランザクションに直接依存しないように汎用化し、ストレージなどの非データベース書き込み処理にも対応できるようにしました。トランザクションは `context.Context` を介して渡されるように変更されました。

### 実装済みファイル:
* `pkg/batch/core/application/port/interfaces.go`
* `pkg/batch/core/tx/tx.go`
* `pkg/batch/engine/step/chunk_step.go` （概念的な変更）
* データベース固有の `ItemWriter` 実装（概念的な変更）

### 実装詳細:
* **`pkg/batch/core/application/port/interfaces.go`**:
    * `ItemWriter` インターフェースの `Write` メソッドのシグネチャは `Write(ctx context.Context, items []I) error` に変更され、`tx tx.Tx` 引数は削除されました。
* **`pkg/batch/core/tx/tx.go`**:
    * `contextKey` 型と `TransactionContextKey` 定数が追加されました。
    * `ContextWithTx(ctx context.Context, tx Tx) context.Context` ヘルパー関数が追加されました。
    * `TxFromContext(ctx context.Context) (Tx, bool)` ヘルパー関数が追加されました。
* **`pkg/batch/engine/step/chunk_step.go` （概念的な変更）**:
    * `ChunkStep` の `Execute` メソッド内で、トランザクションを開始した後、その `tx.Tx` オブジェクトを `tx.ContextWithTx` を使用して `context.Context` に格納し、そのコンテキストを `ItemWriter.Write` メソッドに渡すように変更されました。
* **データベース固有の `ItemWriter` 実装（概念的な変更）**:
    * データベースに書き込む `ItemWriter` の実装は、`Write` メソッド内で `tx.TxFromContext` を使用して `context.Context` から `tx.Tx` オブジェクトを取得し、それを使用してデータベース操作を実行するように変更されました。

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

*   `ComponentBuilder` は `map[string]coreAdapter.ResourceProvider` を受け取るように変更され、各 `ItemReader`、`ItemProcessor`、`ItemWriter` の実装は、このマップから必要な `ResourceProvider` を特定し、そこから `ResourceConnection` を取得する具体的なパターンが確立されました。
*   例えば、`ItemWriter` が特定のデータベース接続を必要とする場合、`resourceProviders["database"]` のようにマップから `database.DBProvider` を取得し、その `GetConnection` メソッドを呼び出すといったロジックが実装されています。

### 4.4. フレームワークマイグレーションフックにおける接続の堅牢性

*   `runFrameworkMigrationsHook` 関数内で、データベース接続の取得方法は `database.DBProvider.GetConnection` を使用するように変更されました。
*   `DBConnectionResolver.ResolveDBConnection` が接続の有効性保証と再確立の責任を持つ一方で、`database.DBProvider.GetConnection` も同様の堅牢性を提供することが確認されています。

### 4.5. ItemWriter インターフェースの汎用性

*   `ItemWriter` インターフェースの `Write` メソッドはトランザクションから独立し、より汎用的な `GetTargetResourceName()` および `GetResourcePath()` メソッドが導入されました。
*   これにより、データベースだけでなく、ストレージなどの非データベースリソースをターゲットとする `ItemWriter` の実装が可能になり、インターフェースの汎用性が向上しました。

### 4.6. ResourceProvider の登録と設定

*   `JobFactory` が `map[string]coreAdapter.ResourceProvider` を受け取るように変更され、これらの `ResourceProvider` インスタンスは Fx の `fx.Provide` と `fx.Group` を使った登録メカニズムによって構築され、フレームワークに登録されるようになりました。

### 4.7. ResourceConnection のライフサイクル管理

*   `ResourceProvider` から取得した `ResourceConnection` の `Close()` メソッドを誰が、いつ呼び出すのか、その責任範囲を明確にする必要があります。特に、`ItemReader` や `ItemWriter` が `Open()` で接続を取得し、`Close()` で解放するパターンが想定されますが、`ChunkStep` のような上位のコンポーネントが接続のライフサイクルを管理する必要があるか、あるいは `ResourceProvider` 自体が接続プールを管理し、明示的な `Close()`
が不要な場合があるかなど、詳細な設計が必要です。

### 4.8. 非データベースリソースにおけるトランザクション類似の概念

*   `ItemWriter` が汎用化され、`tx.Tx` を直接受け取らなくなるものの、S3 などの非データベースリソースに対する書き込みにおいて、データベースのトランザクション（ACID 特性）に相当する「一貫性」や「アトミック性」をどのように保証するのか、その戦略を検討する必要があります。`tx.Tx`
インターフェースが依然としてデータベース操作に特化しているため、異なるリソースタイプ間での整合性確保には、より上位の調整メカニズムや、各リソースタイプに応じた異なるアプローチが必要となる可能性があります。
