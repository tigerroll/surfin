# 技術設計ドキュメント：SqlBulkWriter

## 1. 概要
Go言語製バッチフレームワークSurfinにおける、汎用的なデータベース書き込みコンポーネントである`SqlBulkWriter`の設計について記述する。本Writerは、Spring Batchの`JdbcBatchItemWriter`の思想をGo言語の`database/sql`パッケージに適用し、大量データの効率的なバルク書き込みを実現する。

## 2. 設計の背景とJdbcBatchItemWriterからの着想
Spring Batchの`JdbcBatchItemWriter`は、指定されたSQLステートメントとバッチサイズに基づいて、複数のアイテムをまとめてデータベースに書き込むことで、I/Oオーバーヘッドを削減し、書き込みパフォーマンスを向上させる。この設計思想をGo言語に持ち込むことで、Surfinにおいても同様の効率性と信頼性を持つデータ書き込み機能を提供する。

## 3. SqlBulkWriterの主要設計
`SqlBulkWriter`は、`github.com/tigerroll/surfin/pkg/batch/core/application/port/interfaces.go`で定義される`ItemWriter`インターフェースを実装する。

### 3.1. 配置パス
`pkg/batch/component/step/writer/sql_bulk_writer.go`

### 3.2. 構造体定義
```go
type SqlBulkWriter[T any] struct {
	db         *sql.DB                 // データベース接続
	sql        string                  // 実行するSQLステートメント（例: INSERT INTO table (col1, col2) VALUES (?, ?)）
	bulkSize   int                     // 一度にコミットするアイテムの最大数
	itemToArgs func(T) ([]any, error)  // T型のアイテムをSQLの引数スライスに変換する関数
	name       string                  // Writerの名前。ロギングなどに使用されます。
}
```

### 3.3. コンストラクタ
`NewSqlBulkWriter`関数は、`SqlBulkWriter`の新しいインスタンスを作成する。

```go
func NewSqlBulkWriter[T any](db *sql.DB, name string, sql string, bulkSize int, itemToArgs func(T) ([]any, error)) *SqlBulkWriter[T] {
	return &SqlBulkWriter[T]{
		db:         db,
		name:       name,
		sql:        sql,
		bulkSize:   bulkSize,
		itemToArgs: itemToArgs,
	}
}
```
*   `db *sql.DB`: データベース接続。
*   `name string`: Writerのユニークな名前。ロギングなどに使用される。
*   `sql string`: 実行するSQLステートメント。通常は`INSERT`または`UPSERT`文で、プレースホルダ（`?`）を含む。
*   `bulkSize int`: 一度のデータベース操作で処理するアイテムの最大数。
*   `itemToArgs func(T) ([]any, error)`: ジェネリクス型`T`のデータを受け取り、SQLステートメントのプレースホルダにバインドするための`[]any`スライスを返す関数。

### 3.4. ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Writerの初期化。
*   **動作**: 現時点では特別な初期化は不要だが、将来的にデータベース固有の最適化（例: プリペアドステートメントの準備）が必要になった場合のために用意する。

#### `Write(ctx context.Context, items []T) error`
*   **役割**: 受け取ったデータ項目（`items`スライス）をデータベースに書き込む。
*   **動作**:
    1.  `items`スライスを、設定された`bulkSize`ごとにチャンクに分割する。
    2.  各チャンクに対してデータベーストランザクションを開始する。
    3.  チャンク内の各アイテムについて、`itemToArgs`関数を使用してSQL引数を生成する。
    4.  生成された引数を用いて、`sql`フィールドに指定されたSQLステートメントを`db.ExecContext`で実行する。この際、パフォーマンスのためにプリペアドステートメントを使用することが推奨される。
    5.  チャンク内のすべてのアイテムの処理が完了したら、トランザクションをコミットする。
    6.  途中でエラーが発生した場合は、トランザクションをロールバックし、`exception.NewBatchError`を返す。
*   **注意点**: `database/sql`パッケージにはSpring Batchの`addBatch`/`executeBatch`のような直接的なバッチAPIはない。そのため、トランザクション内で個々の`ExecContext`をループ実行するか、複数の`VALUES`句を持つ単一の`INSERT`ステートメントを動的に構築して実行するアプローチが考えられる。汎用性を考慮すると、トランザクション内でのループ実行が一般的である。

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソースを解放する。
*   **動作**: `Open`で確保したリソース（例: プリペアドステートメント）があればここでクローズする。

## 4. 信頼性の担保
*   **バルク処理**: `bulkSize`を設定することで、ネットワークI/Oとデータベースの負荷を軽減し、大量データの書き込みパフォーマンスを向上させる。
*   **トランザクション**: 各バルク操作はトランザクション内で実行され、原子性（All or Nothing）を保証する。これにより、部分的な書き込みによるデータ不整合を防ぐ。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証し、ランタイムエラーのリスクを低減する。
*   **エラーハンドリング**: データベース操作中に発生したエラーは、Surfinの`BatchError`としてラップされ、適切なモジュール情報とリトライ/スキップ可能性が付与される。
