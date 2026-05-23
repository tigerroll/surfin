# CsvCursorReader の実装

## 1. 概要

* Go言語製バッチフレームワークSurfinにおける、汎用的なCSV読み込みコンポーネントである `CsvCursorReader` の実装について説明します。
* 本Readerは、`SqlCursorReader` と同様の設計思想に基づき、`io.Reader` を抽象化することで、ローカルファイルや外部APIのレスポンスなど、多様なデータソースから大規模なCSVデータを効率的かつ再開可能に読み込むことを目的としています。

## 2. 設計の背景と目的

* バッチ処理において、CSVファイルは頻繁に利用されるデータソースです。しかし、ファイル全体をメモリにロードする手法は大規模データセットにおいてメモリ不足（OOM）を引き起こすリスクがあります。
* `CsvCursorReader` は、`encoding/csv` を利用したストリーミング読み込みを行うことでメモリ消費量を一定（O(1)）に保ちます。
* また、`io.Reader` をインターフェースとして採用することで、ファイルシステムだけでなく、HTTPレスポンス等のネットワークストリームも同一のロジックで処理可能にしています。

## 3. CsvCursorReaderの主要設計

### 3.1 配置パス
`pkg/batch/component/step/reader/csv_cursor_reader.go`

### 3.2 構造体定義
```go
type CsvCursorReader[T any] struct {
	reader    io.Reader                  // reader is the data source (e.g., *os.File, http.Response.Body).
	csvReader *csv.Reader                // csvReader is the parser for CSV data.
	name      string                     // name is the unique name of the reader.
	readCount int                        // readCount is the current number of items read, used for restartability.
	mapper    func([]string) (T, error)  // mapper is a function that maps a CSV record to a value of type T.
	ec        model.ExecutionContext     // ec is the internal ExecutionContext for storing and retrieving state.
}
```

### 3.3 コンストラクタ
`NewCsvCursorReader` 関数は、`CsvCursorReader` の新しいインスタンスを作成します。
```go
func NewCsvCursorReader[T any](reader io.Reader, name string, mapper func([]string) (T, error)) *CsvCursorReader[T] {
	return &CsvCursorReader[T]{
		reader: reader,
		name:   name,
		mapper: mapper,
	}
}
```

### 3.4 ライフサイクルメソッドの実装

#### `Open(ctx context.Context, ec model.ExecutionContext) error`
*   **役割**: Readerの初期化とCSVリーダーのセットアップ。
*   **再開性への対応**: `ExecutionContext` から `readCount` を取得し、その行数分だけ `csvReader.Read()` を空読みすることで、中断された位置から読み込みを再開します。

#### `Read(ctx context.Context) (T, error)`
*   **役割**: 次のCSVレコードを読み込み、`mapper` を通じて型 `T` に変換します。
*   **動作**:
    1. `csvReader.Read()` で1行取得。
    2. `mapper` 関数で変換。
    3. `readCount` をインクリメントし、`ExecutionContext` を更新。

#### `Close(ctx context.Context) error`
*   **役割**: 使用したリソース（ファイル等）の解放。
*   **動作**: `reader` が `io.Closer` を実装している場合、`Close()` を呼び出します。

#### `GetExecutionContext` / `SetExecutionContext`
*   **役割**: 再開性のための状態管理。`readCount` を `ExecutionContext` と同期させます。

## 4. 信頼性の担保
*   **ストリーミング処理**: `io.Reader` を通じて1行ずつ処理するため、ファイルサイズに依存しない安定したメモリ消費を実現します。
*   **再開性**: `ExecutionContext` と `readCount` を利用することで、障害発生時でも中断箇所から処理を再開できます（※ネットワークストリームの場合は最初からの再取得を前提とする）。
*   **柔軟性**: `io.Reader` を受け取る設計により、ローカルファイル、S3からのダウンロード、HTTP APIレスポンスなど、あらゆるデータソースに対応可能です。
*   **型安全性**: Goのジェネリクスを使用することで、コンパイル時に型の一貫性を保証します。
