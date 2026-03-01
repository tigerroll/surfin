# SqlCursorReader の実装

## 1. 概要
Go言語製バッチフレームワークSurfinにおける、汎用的なデータベース読み込みコンポーネントである`SqlCursorReader`の実装について説明します。本Readerは、Spring Batchの`JdbcCursorItemReader`の思想をGo言語の`database/sql`パッケージに適用し、大規模データセットの効率的かつ再開可能な読み込みを実現しています。

## 2. 設計の背景とJdbcCursorItemReaderからの着想
Spring Batchの`JdbcCursorItemReader`は、大量のデータをメモリに一度にロードすることなく、データベースカーソルを利用して1行ずつ効率的に読み込むことを特徴としています。また、バッチ処理の重要な要件である再開性（Restartability）をサポートし、中断された処理を前回の読み込み位置から再開できます。
この堅牢な設計思想をGo言語に持ち込むことで、Surfinにおいても同様の信頼性とパフォーマンスを持つデータ読み込み機能を提供しています。

## 3. SqlCursorReaderの主要設計
`SqlCursorReader`は、`github.com/tigerroll/surfin/pkg/batch/core/application/port/interfaces.go`で定義される`ItemReader`インターフェースを実装しています。

### 3.1. 配置パス
`pkg/batch/component/step/reader/sql_cursor_reader.go`

### 3.2. 構造体定義
```go
type SqlCursorReader[T any] struct {
	db        *sql.DB                    // db is the database connection.
	name      string                     // name is the unique name of the reader, used for storing state in the ExecutionContext.
	baseQuery string                     // baseQuery is the SQL query without OFFSET/LIMIT clauses.
	baseArgs  []any                      // baseArgs are the arguments for the base query.
	mapper    func(*sql.Rows) (T, error) // mapper is a function that maps a *sql.Rows to a value of type T.
	rows      *sql.Rows                  // rows is the result set from the executed query.
	readCount int                        // readCount is the current number of items read, used for restartability.
	ec        model.ExecutionContext     // ec is the internal ExecutionContext for storing and retrieving state.
}
```

### 3.3. コンストラクタ
`NewSqlCursorReader`関数は、`SqlCursorReader`の新しいインスタンスを作成します。
```go
func NewSqlCursorReader[T any](db *sql.DB, name string, query string, args []any, mapper func(*sql.Rows) (T, error)) *SqlCursorReader[T] {
	return &SqlCursorReader[T]{
		db:        db,
		name:      name,
		baseQuery: query,
		baseArgs:  args,
		mapper:    mapper,
	}
}
```
*   `db *sql.DB`: データベース接続。
*   `name string`: Readerのユニークな名前。再開性のため`ExecutionContext`に状態を保存する際のキーとして使用されます。
*   `query string`: 実行するSQLクエリのベース（`OFFSET`句を含まない）。
*   `args []any`: ベースクエリのプレースホルダにバインドする引数。
*   `mapper func(*sql.Rows) (T, error)`: `*sql.Rows`からジェネリクス型`T`のデータにマッピングする関数。
```

*   `mapper func(*sql.Rows) (T, error)`: `*sql.Rows`からジェネリクス型`T`のデータにマッピングする関数。

### 3.4. ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Readerの初期化とデータベースクエリの実行。
*   **再開性への対応**: `ec` (ExecutionContext) から前回の`readCount`（読み込み済みアイテム数）を取得します。この`readCount`をSQLクエリの`OFFSET`として利用し、中断された位置からデータの読み込みを再開します。
    *   `readCountKey`は`ReaderName.readCount`のような形式で`ExecutionContext`から取得されます。
    *   `ec.GetInt(readCountKey)` を使用して `startOffset` を取得し、`r.readCount` に設定します。
    *   SQLクエリは`fmt.Sprintf("%s OFFSET ?", r.baseQuery)`のように動的に`OFFSET`句が追加され、`r.readCount`がバインドされます。
    *   `db.QueryContext(ctx, query, args...)` を使用してクエリを実行します。
*   **注意点**: データベースの種類によって`OFFSET`の構文が異なる場合があるため、汎用的な実装には注意が必要です。

```go
func (r *SqlCursorReader[T]) Open(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec // Store the provided ExecutionContext
	readCountKey := r.name + ".readCount"

	startOffset, found := r.ec.GetInt(readCountKey)
	if found {
		r.readCount = startOffset
	} else {
		r.readCount = 0 // Initialize if not found
	}

	query := r.baseQuery
	args := r.baseArgs

	if r.readCount > 0 { // Use r.readCount for offset
		query = fmt.Sprintf("%s OFFSET ?", r.baseQuery)
		args = append(args, r.readCount)
		logger.Infof("SqlCursorReader '%s': Resuming from offset %d. Query: %s", r.name, r.readCount, query)
	} else {
		logger.Infof("SqlCursorReader '%s': Starting new read. Query: %s", r.name, query)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return exception.NewBatchError("reader", fmt.Sprintf("Failed to execute query for SqlCursorReader '%s'", r.name), err, false, false)
	}
	r.rows = rows

	return nil
}
```

#### `Read() (T, error)`
*   **役割**: 次のデータ項目を読み込む。
*   **動作**:
    1.  `r.rows` が `nil` の場合、リーダーが初期化されていないことを示すエラーを返します。
    2.  `r.rows.Next()` を呼び出して次の行に進みます。
    3.  `r.rows.Next()` が `false` を返した場合、`r.rows.Err()` をチェックし、エラーがあればそれを返し、なければ `io.EOF` を返してデータの終了を示します。
    4.  `r.mapper` 関数を使用して `*sql.Rows` から `T` 型のデータを生成します。
    5.  `r.readCount` をインクリメントし、`r.ec` に `r.name + ".readCount"` のキーで保存します。
*   **注意点**: `Read` メソッドは `ctx context.Context` を引数に取りますが、現在の実装では `ctx` は直接使用されていません。

```go
func (r *SqlCursorReader[T]) Read(ctx context.Context) (T, error) {
	var item T
	if r.rows == nil {
		return item, exception.NewBatchError("reader", fmt.Sprintf("SqlCursorReader '%s': Reader not opened or already closed.", r.name), errors.New("reader not initialized"), false, false)
	}

	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return item, exception.NewBatchError("reader", fmt.Sprintf("Error during row iteration for SqlCursorReader '%s'", r.name), err, false, false)
		}
		return item, io.EOF
	}

	mappedItem, err := r.mapper(r.rows)
	if err != nil {
		return item, exception.NewBatchError("reader", fmt.Sprintf("Failed to map row for SqlCursorReader '%s'", r.name), err, false, false)
	}

	r.readCount++;
	r.ec.Put(r.name+".readCount", r.readCount) // Update the internal EC

	return mappedItem, nil
}
```

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソース（データベースカーソルなど）を解放する。
*   **動作**: `r.rows.Close()` を呼び出し、エラーがあれば `exception.NewBatchError` でラップして返します。`r.rows` を `nil` に設定します。
*   **注意点**: `ItemReader`インターフェースの`Close`メソッドは`ExecutionContext`を受け取らないため、このメソッド内で直接`ExecutionContext`に読み込み位置を保存することはありません。読み込み位置の保存は、フレームワーク（例: `StepExecutor`）が`Read()`の呼び出し回数を追跡し、`StepExecution`の`ExecutionContext`に保存する責任を負います。

```go
func (r *SqlCursorReader[T]) Close(ctx context.Context) error {
	if r.rows != nil {
		err := r.rows.Close()
		if err != nil {
			return exception.NewBatchError("reader", fmt.Sprintf("Failed to close rows for SqlCursorReader '%s'", r.name), err, false, false)
		}
		r.rows = nil // Set to nil after closing.
	}
	logger.Infof("SqlCursorReader '%s': Resources closed.", r.name)
	return nil
}

#### `GetExecutionContext(ctx context.Context) (model.ExecutionContext, error)`
*   **役割**: Readerの現在の`ExecutionContext`を取得します。再開性のために使用されます。
*   **動作**: 内部に保存されている `r.ec` を返します。初期化されていない場合は空の `ExecutionContext` を返します。

```go
func (r *SqlCursorReader[T]) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	if r.ec == nil {
		return model.NewExecutionContext(), nil // Return empty if not initialized
	}
	return r.ec, nil
}
```

#### `SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Readerの`ExecutionContext`を設定します。再開性のために状態を復元するために使用されます。
*   **動作**: 提供された `ec` を `r.ec` に保存し、`ec` から `readCount` を読み込んで `r.readCount` を更新します。

```go
func (r *SqlCursorReader[T]) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	r.ec = ec
	readCountKey := r.name + ".readCount"
	if val, found := ec.GetInt(readCountKey); found {
		r.readCount = val
	} else {
		r.readCount = 0
	}
	return nil
}
```
```

## 4. 信頼性の担保
*   **カーソルベース**: 大規模データセットでもメモリ効率を保ちながら処理が可能です。
*   **再開性**: `ExecutionContext`と`OFFSET`を利用することで、障害発生時でも中断箇所から処理を再開できます。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証し、ランタイムエラーのリスクを低減します。
