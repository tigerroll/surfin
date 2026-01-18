# SQLリポジトリの命名抽象化リファクタリング計画

## 1. 概要

### 1.1. 現状と課題
現在、`pkg/batch/infrastructure/repository/sql/repository.go` 内の `JobRepository` 実装は、`GORMJobRepository` という名称を使用しており、コード内のコメントや文字列リテラルにも `GORM` という特定のORM名が含まれています。
しかし、この実装は `adaptor.DBConnectionResolver` や `tx.TransactionManager` といった抽象化されたインターフェースを介してデータベース操作を行っており、GORMの具体的な型に直接依存しているわけではありません。
このため、`infrastructure` レイヤーが特定のORMに強く依存しているかのような印象を与え、不本意な依存関係となっています。

### 1.2. 目的
このリファクタリングの目的は、`pkg/batch/infrastructure/repository/sql` パッケージが特定のORM（GORM）に依存しているかのような命名を排除し、より汎用的な「SQLリポジトリ」として機能することを明確にすることです。これにより、コードの意図を明確にし、将来的なORMの変更やDBアクセスライブラリの切り替えに対する柔軟性を高めます。

### 1.3. 変更範囲
この変更は、主に命名とコメントの修正に限定され、既存の機能や動作に影響を与えません。

## 2. 実装計画

### 2.1. 影響ファイル
*   `pkg/batch/infrastructure/repository/sql/repository.go`
*   `pkg/batch/infrastructure/repository/sql/schema.go` (コメント修正のみ)
*   `pkg/batch/infrastructure/tx/manager.go` (コメント修正のみ)

### 2.2. 変更内容と手順

以下の手順で変更を実施します。各ステップは既存の動作を維持するように注意深く行われます。

1.  **`pkg/batch/infrastructure/repository/sql/repository.go` の変更**
    *   `GORMJobRepository` 構造体の名称を `SQLJobRepository` に変更します。
        ```go
        // 変更前: type GORMJobRepository struct {
        // 変更後: type SQLJobRepository struct {
        ```
    *   `NewGORMJobRepository` 関数の名称を `NewSQLJobRepository` に変更します。
        ```go
        // 変更前: func NewGORMJobRepository(...) { ... return &GORMJobRepository{...} }
        // 変更後: func NewSQLJobRepository(...) { ... return &SQLJobRepository{...} }
        ```
    *   ファイル内の文字列リテラル `GORMJobRepository` を `SQLJobRepository` に変更します。これは主に `const op = "GORMJobRepository.SaveJobInstance"` のような操作名文字列に適用されます。
    *   コメントアウトされている `getDBContext` メソッド内のGORMへの言及を削除します。
        ```go
        // 変更前: // getDBContext retrieves a GORM DB instance from a DBConnection and sets the Context.
        // 変更後: // getDBContext retrieves a DB instance from a DBConnection and sets the Context.

        // 変更前: gormDB, err := database.GetGormDBFromConnection(r.dbConn)
        // 変更後: db, err := database.GetDBFromConnection(r.dbConn) // この行はコメントアウトされているため、GORMの言及を削除するのみ

        // 変更前: return nil, exception.NewBatchError("GORMJobRepository", "Failed to get GORM DB connection", err, false, false)
        // 変更後: return nil, exception.NewBatchError(op, "Failed to get DB connection", err, false, false)

        // 変更前: return gormDB.WithContext(ctx), nil
        // 変更後: return db.WithContext(ctx), nil // この行はコメントアウトされているため、GORMの言及を削除するのみ
        ```
    *   `var _ repository.JobRepository = (*GORMJobRepository)(nil)` の型アサーションを `(*SQLJobRepository)(nil)` に変更します。

2.  **`pkg/batch/infrastructure/repository/sql/schema.go` の変更**
    *   GORMへの言及を含むコメントを修正します。
        ```go
        // 変更前: // StepExecutions []*StepExecutionEntity // Removed to avoid GORM schema parsing errors.
        // 変更後: // StepExecutions []*StepExecutionEntity // Removed to avoid ORM schema parsing errors.

        // 変更前: // JobExecution *JobExecutionEntity // Removed to avoid GORM schema parsing errors.
        // 変更後: // JobExecution *JobExecutionEntity // Removed to avoid ORM schema parsing errors.
        ```

3.  **`pkg/batch/infrastructure/tx/manager.go` の変更**
    *   GORMへの言及を含むコメントを修正します。
        ```go
        // 変更前: // The implementation of GormTxAdapter and gormTransactionManager now resides in pkg/batch/adaptor/database/adapter.go,
        // 変更後: // The implementation of TxAdapter and TransactionManager now resides in pkg/batch/adaptor/database/adapter.go,
        ```

### 2.3. テスト
*   上記の変更後、全ての既存の単体テストおよび統合テストがパスすることを確認します。
*   特に、`pkg/batch/infrastructure/repository/sql` パッケージに関連するテストが、命名変更後も正しく動作することを確認します。
*   `go test ./...` コマンドを実行し、全てのテストが成功することを確認します。

## 3. 考慮事項

*   このリファクタリングは、`pkg/batch/infrastructure/repository/sql` パッケージの命名の抽象化に限定されます。
*   `pkg/batch/adaptor/database/gorm` パッケージ内のGORMへの直接的な依存は、そのパッケージの責務であり、このタスクの範囲外とします。
*   機能的な変更やパフォーマンスへの影響はありません。
