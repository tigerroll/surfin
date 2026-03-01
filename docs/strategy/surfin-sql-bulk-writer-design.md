# SqlBulkWriter の実装

## 1. 概要
Go言語製バッチフレームワークSurfinにおける、汎用的なデータベース書き込みコンポーネントである`SqlBulkWriter`の実装について説明します。本Writerは、Spring Batchの`JdbcBatchItemWriter`の思想をGo言語の`database/sql`パッケージに適用し、大量データの効率的なバルク書き込みを実現しています。

## 2. 設計の背景とJdbcBatchItemWriterからの着想
Spring Batchの`JdbcBatchItemWriter`は、指定されたSQLステートメントとバッチサイズに基づいて、複数のアイテムをまとめてデータベースに書き込むことで、I/Oオーバーヘッドを削減し、書き込みパフォーマンスを向上させます。この設計思想をGo言語に持ち込むことで、Surfinにおいても同様の効率性と信頼性を持つデータ書き込み機能を提供しています。

## 3. SqlBulkWriterの主要設計
`SqlBulkWriter`は、`github.com/tigerroll/surfin/pkg/batch/core/application/port/interfaces.go`で定義される`ItemWriter`インターフェースを実装しています。

### 3.1. 配置パス
`pkg/batch/component/step/writer/sql_bulk_writer.go`

### 3.2. 構造体定義
```go
type SqlBulkWriter[T any] struct {
	name                 string                 // name is the unique name of the writer instance, used for logging and resource identification.
	bulkSize             int                    // bulkSize is the maximum number of items to process in a single database operation (chunk size).
	tableName            string                 // tableName is the name of the target database table where items will be written.
	conflictColumns      []string               // conflictColumns are the columns used for conflict resolution during UPSERT operations (e.g., primary keys).
	updateColumns        []string               // updateColumns are the columns to update if a conflict occurs during an UPSERT operation (empty for DO NOTHING).
	stepExecutionContext model.ExecutionContext // stepExecutionContext holds a reference to the Step's ExecutionContext, primarily for state management by the framework.
}
```

### 3.3. コンストラクタ
`NewSqlBulkWriter`関数は、`SqlBulkWriter`の新しいインスタンスを作成する。

```go
func NewSqlBulkWriter[T any](name string, bulkSize int, tableName string, conflictColumns []string, updateColumns []string) *SqlBulkWriter[T] {
	return &SqlBulkWriter[T]{
		name:            name,
		bulkSize:        bulkSize,
		tableName:       tableName,
		conflictColumns: conflictColumns,
		updateColumns:   updateColumns,
	}
}
```
*   `name string`: Writerのユニークな名前。ロギングなどに使用される。
*   `bulkSize int`: 一度のデータベース操作で処理するアイテムの最大数。
*   `tableName string`: 対象となるテーブル名。
*   `conflictColumns []string`: UPSERT時の競合カラム（例: プライマリキー）。
*   `updateColumns []string`: UPSERT時に更新するカラム（DO NOTHINGの場合は空）。

#### `GetTargetResourceName() string`
*   **役割**: このWriterが書き込むターゲットリソースのユニークな名前を返します。
*   **動作**: コンストラクタで設定された `name` の値を返します。

#### `GetResourcePath() string`
*   **役割**: このWriterが書き込むターゲットリソース内のパスまたは識別子を返します。
*   **動作**: コンストラクタで設定された `tableName` の値を返します。

### 3.4. ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Writerの初期化。
*   **動作**: `SqlBulkWriter`はデータベース接続を直接保持しないため、ここでは特別な初期化は不要です。

#### `Write(ctx context.Context, items []T) error`
*   **役割**: 受け取ったデータ項目（`items`スライス）をデータベースに書き込む。
*   **動作**:
    1.  `items`スライスを、設定された`bulkSize`ごとにチャンクに分割する。
    2.  `tx.TxFromContext(ctx)` を使用して、`context`からフレームワークが管理する`Tx`インスタンスを取得する。
    3.  取得した`Tx`インスタンスの`ExecuteUpsert`メソッドに各チャンクを渡し、バルクUPSERTを実行する。
    4.  `ExecuteUpsert`内でエラーが発生した場合は、`exception.NewBatchError`を返す。
*   **注意点**: `SqlBulkWriter`は、フレームワークが提供するトランザクションに**参加**します。自身でトランザクションを開始・コミットしません。

```go
func (w *SqlBulkWriter[T]) Write(ctx context.Context, items []T) error {
	if len(items) == 0 {
		return nil // Do nothing if there are no items to write.
	}

	// Retrieve the transaction from the context.
	currentTx, ok := tx.TxFromContext(ctx)
	if !ok {
		return exception.NewBatchError("writer", "transaction not found in context for SqlBulkWriter", nil, false, false)
	}

	// Divide the items slice into chunks based on bulkSize.
	for i := 0; i < len(items); i += w.bulkSize {
		end := i + w.bulkSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		// Perform bulk UPSERT using the transaction from context.
		// The Tx.ExecuteUpsert method handles its own statement preparation and execution.
		_, err := currentTx.ExecuteUpsert(ctx, chunk, w.tableName, w.conflictColumns, w.updateColumns)
		if err != nil {
			return exception.NewBatchError("writer", fmt.Sprintf("Failed to bulk upsert data for SqlBulkWriter '%s' (chunk start index %d)", w.name, i), err, false, false)
		}

		logger.Debugf("SqlBulkWriter '%s': Wrote %d items in chunk (start index %d).", w.name, len(chunk), i)
	}

	logger.Infof("SqlBulkWriter '%s': Successfully wrote all %d items.", w.name, len(items))
	return nil
}
```

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソースを解放する。
*   **動作**: `SqlBulkWriter`はデータベース接続を直接保持しないため、ここでは特別なリソース解放は不要です。

## 4. 信頼性の担保
*   **バルク処理**: `bulkSize`を設定することで、ネットワークI/Oとデータベースの負荷を軽減し、大量データの書き込みパフォーマンスを向上させます。
*   **トランザクション**: 各バルク操作はフレームワークが提供するトランザクション内で実行され、原子性（All or Nothing）を保証します。これにより、部分的な書き込みによるデータ不整合を防ぎます。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証し、ランタイムエラーのリスクを低減します。
*   **エラーハンドリング**: データベース操作中に発生したエラーは、Surfinの`BatchError`としてラップされ、適切なモジュール情報とリトライ/スキップ可能性が付与されます。
