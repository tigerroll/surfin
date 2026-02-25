# SqlBulkWriter 実装手順

このドキュメントは、`pkg/batch/component/step/writer/sql_bulk_writer.go` の実装を進めるためのガイドです。

## 1. 概要

`SqlBulkWriter` は、Go言語製バッチフレームワークSurfinにおいて、大量データを効率的にデータベースに書き込むためのコンポーネントです。Spring Batchの`JdbcBatchItemWriter`の思想に基づき、バルク書き込みとトランザクション管理を統合します。

## 2. 実装対象ファイル

`pkg/batch/component/step/writer/sql_bulk_writer.go`

## 3. 実装ステップ

### 3.1. 必要なパッケージのインポート

`sql_bulk_writer.go` の冒頭で、必要なパッケージをインポートします。

```go
import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	"github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception" // パスを修正
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"   // パスとパッケージ名を修正
)
```

### 3.2. 構造体定義の確認

`docs/strategy/surfin-sql-bulk-writer-design.md` に記載されている `SqlBulkWriter` 構造体定義が正しいことを確認します。
```go
type SqlBulkWriter[T any] struct {
	bulkSize   int                     // 一度にコミットするアイテムの最大数
	name       string                  // Writerの名前。ロギングなどに使用されます。
	// 以下のフィールドは、TxインターフェースのExecuteUpsert/ExecuteUpdateを使用するために必要
	tableName       string   // 対象となるテーブル名
	conflictColumns []string // UPSERT時の競合カラム（例: プライマリキー）
	updateColumns   []string // UPSERT時に更新するカラム（DO NOTHINGの場合は空）
}
```

### 3.3. コンストラクタ `NewSqlBulkWriter` の実装

`SqlBulkWriter` の新しいインスタンスを作成するコンストラクタを実装します。
```go
// NewSqlBulkWriter は SqlBulkWriter の新しいインスタンスを作成します。
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

### 3.4. `ItemWriter` インターフェースの実装確認

`SqlBulkWriter` が `port.ItemWriter` インターフェースを実装していることをコンパイル時に確認するための宣言を追加します。

```go
// SqlBulkWriter が port.ItemWriter インターフェースを実装していることをコンパイル時に確認します。
var _ port.ItemWriter[any] = (*SqlBulkWriter[any])(nil)
```

### 3.5. `Open` メソッドの実装

`Open` メソッドはWriterの初期化を行います。ここでは、SQLステートメントのプリペアを行います。

```go
// Open は Writer を初期化します。
// `Tx`インターフェースが内部でステートメントを管理するため、ここでは特別な初期化は不要です。
func (w *SqlBulkWriter[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	logger.Infof("SqlBulkWriter '%s': Opened.", w.name)
	return nil
}
```

### 3.6. `Write` メソッドの実装

`Write` メソッドは、受け取ったアイテムのリストをデータベースにバルク書き込みします。トランザクション内で個々の `ExecContext` をループ実行するアプローチを採用します。

```go
// Write は受け取ったデータ項目をデータベースに書き込みます。
// `SqlBulkWriter`は、フレームワークが提供するトランザクションに**参加**します。自身でトランザクションを開始・コミットしません。
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

### 3.7. `Close` メソッドの実装

`Close` メソッドは使用したリソースを解放します。ここでは、プリペアされたステートメントをクローズします。

```go
// Close は使用したリソースを解放します。
// `Tx`インターフェースが内部でリソースを管理するため、ここでは特別なリソース解放は不要です。
func (w *SqlBulkWriter[T]) Close(ctx context.Context) error {
	logger.Infof("SqlBulkWriter '%s': Closed.", w.name)
	return nil
}
```

## 4. テストと検証

実装後、以下の点を考慮してテストと検証を行います。

*   **正常系**: 少量データ、大量データでの書き込みが正しく行われるか。
*   **異常系**:
    *   データベース接続エラー
    *   SQL実行エラー（例: 制約違反）
    *   `itemToArgs` 関数でのエラー
*   **トランザクション**: エラー発生時にトランザクションが正しくロールバックされるか。
*   **パフォーマンス**: `bulkSize` の値を変えてパフォーマンスを測定し、最適な値を見つける。

## 5. 今後の改善点（オプション）

*   複数の `VALUES` 句を持つ単一の `INSERT` ステートメントの動的構築: より真のバルクインサートを実現するために、このアプローチを検討する。ただし、データベースの種類やSQLの複雑さに依存する。
*   `bulkSize` の動的な調整機能。
