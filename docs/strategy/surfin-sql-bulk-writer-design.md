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
	name       string                  // Writerの名前。ロギングなどに使用されます。
	bulkSize   int                     // 一度にコミットするアイテムの最大数
	// 以下のフィールドは、TxインターフェースのExecuteUpsert/ExecuteUpdateを使用するために必要
	tableName       string   // 対象となるテーブル名
	conflictColumns []string // UPSERT時の競合カラム（例: プライマリキー）
	updateColumns   []string // UPSERT時に更新するカラム（DO NOTHINGの場合は空）
}
```

### 3.3. コンストラクタ
`NewSqlBulkWriter`関数は、`SqlBulkWriter`の新しいインスタンスを作成する。

```go
func NewSqlBulkWriter[T any](name string, bulkSize int, tableName string, conflictColumns []string, updateColumns []string) *SqlBulkWriter[T] {
	return &SqlBulkWriter[T]{
		name:       name,
		bulkSize:   bulkSize,
		tableName: tableName,
		conflictColumns: conflictColumns,
		updateColumns: updateColumns,
	}
}
```
*   `name string`: Writerのユニークな名前。ロギングなどに使用される。
*   `name string`: Writerのユニークな名前。ロギングなどに使用される。
*   `bulkSize int`: 一度のデータベース操作で処理するアイテムの最大数。
*   `tableName string`: 対象となるテーブル名。
*   `conflictColumns []string`: UPSERT時の競合カラム（例: プライマリキー）。
*   `updateColumns []string`: UPSERT時に更新するカラム（DO NOTHINGの場合は空）。

### 3.4. ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Writerの初期化。
*   **動作**: `Tx`インターフェースが内部でステートメントを管理するため、ここでは特別な初期化は不要。

#### `Write(ctx context.Context, items []T) error`
*   **役割**: 受け取ったデータ項目（`items`スライス）をデータベースに書き込む。
*   **動作**:
    1.  `items`スライスを、設定された`bulkSize`ごとにチャンクに分割する。
    2.  `context`からフレームワークが管理する`Tx`インスタンスを取得する。
    3.  各チャンクを`Tx.ExecuteUpsert`メソッドに渡し、バルクUPSERTを実行する。
    4.  `ExecuteUpsert`内でエラーが発生した場合は、`exception.NewBatchError`を返す。
*   **注意点**: `SqlBulkWriter`は、フレームワークが提供するトランザクションに**参加**する。自身でトランザクションを開始・コミットしない。

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソースを解放する。
*   **動作**: `Tx`インターフェースが内部でリソースを管理するため、ここでは特別なリソース解放は不要。

## 4. 信頼性の担保
*   **バルク処理**: `bulkSize`を設定することで、ネットワークI/Oとデータベースの負荷を軽減し、大量データの書き込みパフォーマンスを向上させる。
*   **トランザクション**: 各バルク操作はトランザクション内で実行され、原子性（All or Nothing）を保証する。これにより、部分的な書き込みによるデータ不整合を防ぐ。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証し、ランタイムエラーのリスクを低減する。
*   **エラーハンドリング**: データベース操作中に発生したエラーは、Surfinの`BatchError`としてラップされ、適切なモジュール情報とリトライ/スキップ可能性が付与される。
