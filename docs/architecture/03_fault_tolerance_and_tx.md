# 3. 障害耐性 (Fault Tolerance) とトランザクション管理

Surfin Batch Frameworkは、エンタープライズレベルの障害耐性を提供するために、リトライ、スキップ、および柔軟なトランザクション伝播をサポートします。

## 3.1. チャンク処理における障害耐性

チャンク指向ステップ（`ChunkStep`）は、アイテムレベルおよびチャンクレベルでの障害耐性ロジックを内部に組み込んでいます。

### 3.1.1. アイテムレベルのリトライとスキップ

*   **リトライ (Item Retry)**: `ItemReader` または `ItemProcessor` が `isRetryable=true` のエラー（これは `exception.BatchError` の `IsRetryable()` メソッドによって判断されます。例: 一時的なネットワーク障害）を返した場合、`config.ItemRetryConfig` で設定されたポリシーに基づいてアイテムの読み込み/処理が再試行されます。
*   **スキップ (Item Skip)**: `ItemReader` または `ItemProcessor` が `isSkippable=true` のエラー（これも `exception.BatchError` の `IsSkippable()` メソッドによって判断されます。例: データ形式エラー）を返した場合、`config.ItemSkipConfig` で設定された制限内でアイテムがスキップされ、処理が続行されます。

### 3.1.2. チャンクレベルのリトライと分割 (Chunk Retry / Splitting)

書き込みフェーズでエラーが発生した場合、トランザクションの性質上、チャンク全体が影響を受けます。

1.  **チャンク全体のリトライ**: `ItemWriter` が `isRetryable=true` のエラー（例: デッドロック）を返した場合、トランザクションはロールバックされ、チャンク全体が再実行されます。
2.  **チャンク分割 (Chunk Splitting)**: `ItemWriter` が `isSkippable=true` のエラー（これも `exception.BatchError` の `IsSkippable()` メソッドによって判断されます。例: DB制約違反）を返した場合、以下の手順でエラーアイテムを特定し、スキップします。
    *   チャンク全体をロールバックします。
    *   元のチャンク内のアイテムを**一つずつ**取り出し、個別のトランザクションで再書き込みを試みます。
        このロジックは `ChunkStep` 内部で自動的に処理され、エラーの原因となったアイテムを特定し、それ以外のアイテムの永続化を保証します。
    *   再書き込みで失敗したアイテムはスキップとして記録され、成功したアイテムはコミットされます。

## 3.2. トランザクション伝播属性 (Transaction Propagation)

Stepの実行は、Springのトランザクション伝播属性に類似した以下のモードをサポートします。これらのモードは、`SimpleStepExecutor` によって解釈され、トランザクション境界が確立されます。
これらのモードは、Goの `database/sql` パッケージの `sql.TxOptions` と連携し、トランザクションの振る舞いを細かく制御します。

| 属性 | 動作 | 適用範囲 |
| :--- | :--- | :--- |
| **REQUIRED** (Default) | 既存のトランザクションがあれば参加し、なければ新規に開始します。 | TaskletStep |
| **REQUIRES\_NEW** | 既存のトランザクションをサスペンドし、常に新しいトランザクションを開始します。 | TaskletStep |
| **NESTED** | 既存のトランザクションがあれば、セーブポイント（Savepoint）を作成して実行します。失敗した場合、セーブポイントまでロールバックします。なければ REQUIRED と同様に新規開始します。 | TaskletStep |
| **NOT\_SUPPORTED** | 既存のトランザクションをサスペンドし、トランザクションなしで実行します。 | TaskletStep |
| **SUPPORTS** | 既存のトランザクションがあれば参加し、なければトランザクションなしで実行します。 | TaskletStep |
| **MANDATORY** | 既存のトランザクションが必須です。なければエラーを発生させます。 | TaskletStep |
| **NEVER** | 既存のトランザクションが存在してはなりません。存在すればエラーを発生させます。 | TaskletStep |

### 3.2.1. トランザクションのコンテキスト伝播

トランザクションは、`context.Context` の値として `tx.Tx` インターフェースを格納することで、コンポーネント間で透過的に伝播されます。これにより、各コンポーネントは明示的にトランザクションを管理する必要がなく、疎結合な設計が実現されます。

*   `SimpleStepExecutor` は、トランザクションを開始した場合、`context.WithValue(ctx, "tx", txAdapter)` を使用して新しいコンテキストを作成し、`Step.Execute` に渡します。
*   `JobRepository` や `ItemWriter` は、このコンテキストから `tx.Tx` を取得し、トランザクション内でデータベース操作を実行します。

## 3.3. データベースマイグレーション

データベーススキーマの管理は、`MigrationTasklet` を使用して Job フローの一部として実行されます。

*   `MigrationTasklet` は `golang-migrate/migrate/v4` ライブラリを使用します。
*   マイグレーションの実行後、データベース接続プールが変更されたスキーマを認識できるように、`surfin/pkg/batch/adaptor/database/gorm.DBProvider.ForceReconnect` を呼び出して接続を強制的に再確立するロジックが組み込まれています。
