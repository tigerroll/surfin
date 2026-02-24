# 技術設計ドキュメント：SqlCursorReader

## 1. 概要
Go言語製バッチフレームワークSurfinにおける、汎用的なデータベース読み込みコンポーネントである`SqlCursorReader`の設計について記述する。本Readerは、Spring Batchの`JdbcCursorItemReader`の思想をGo言語の`database/sql`パッケージに適用し、大規模データセットの効率的かつ再開可能な読み込みを実現する。

## 2. 設計の背景とJdbcCursorItemReaderからの着想
Spring Batchの`JdbcCursorItemReader`は、大量のデータをメモリに一度にロードすることなく、データベースカーソルを利用して1行ずつ効率的に読み込むことを特徴とする。また、バッチ処理の重要な要件である再開性（Restartability）をサポートし、中断された処理を前回の読み込み位置から再開できる。
この堅牢な設計思想をGo言語に持ち込むことで、Surfinにおいても同様の信頼性とパフォーマンスを持つデータ読み込み機能を提供する。

## 3. SqlCursorReaderの主要設計
`SqlCursorReader`は、`pkg/batch/core/application/port/interfaces.go`で定義される`ItemReader`インターフェースを実装する。

### 3.1. 配置パス
`pkg/batch/component/step/reader/sql_cursor_reader.go`

### 3.2. 構造体定義
```go
type SqlCursorReader[T any] struct {
	db        *sql.DB                 // データベース接続
	name      string                  // Readerの名前。ExecutionContextに状態を保存する際のキーとして使用されます。
	baseQuery string                  // 実行するSQLクエリのベース（OFFSET/LIMITなし）
	baseArgs  []any                   // ベースクエリの引数
	mapper    func(*sql.Rows) (T, error) // sql.Rows から T 型のデータにマッピングする関数
	rows      *sql.Rows               // 実行されたクエリの結果セット
}
```

### 3.3. コンストラクタ
`NewSqlCursorReader`関数は、`SqlCursorReader`のインスタンスを初期化する。
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
*   `name string`: Readerのユニークな名前。再開性のため`ExecutionContext`に状態を保存する際のキーとして使用される。
*   `query string`: 実行するSQLクエリのベース（`OFFSET`句を含まない）。
*   `args []any`: ベースクエリのプレースホルダにバインドする引数。
*   `mapper func(*sql.Rows) (T, error)`: `*sql.Rows`からジェネリクス型`T`のデータにマッピングする関数。

### 3.4. ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Readerの初期化とデータベースクエリの実行。
*   **再開性への対応**: `ec` (ExecutionContext) から前回の`readCount`（読み込み済みアイテム数）を取得する。この`readCount`をSQLクエリの`OFFSET`として利用し、中断された位置からデータの読み込みを再開する。
    *   `readCountKey`は`ReaderName.readCount`のような形式で`ExecutionContext`から取得されることを想定。
    *   SQLクエリは`fmt.Sprintf("%s OFFSET ?", r.baseQuery)`のように動的に`OFFSET`句が追加され、`startOffset`がバインドされる。
*   **注意点**: データベースの種類によって`OFFSET`の構文が異なる場合があるため、汎用的な実装には注意が必要。

#### `Read() (T, error)`
*   **役割**: 次のデータ項目を読み込む。
*   **動作**: `r.rows.Next()`で次の行に進み、`r.mapper`関数を使用して`*sql.Rows`から`T`型のデータを生成する。データがこれ以上ない場合は`io.EOF`を返す。

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソース（データベースカーソルなど）を解放する。
*   **動作**: `r.rows.Close()`を呼び出す。
*   **注意点**: `ItemReader`インターフェースの`Close`メソッドは`ExecutionContext`を受け取らないため、このメソッド内で直接`ExecutionContext`に読み込み位置を保存することはない。読み込み位置の保存は、フレームワーク（例: `StepExecutor`）が`Read()`の呼び出し回数を追跡し、`StepExecution`の`ExecutionContext`に保存する責任を負う。

## 4. 信頼性の担保
*   **カーソルベース**: 大規模データセットでもメモリ効率を保ちながら処理が可能。
*   **再開性**: `ExecutionContext`と`OFFSET`を利用することで、障害発生時でも中断箇所から処理を再開できる。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証し、ランタイムエラーのリスクを低減する。
