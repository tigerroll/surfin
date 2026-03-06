# Parquet ExportにおけるOOM問題の解決戦略

## 1. 問題の概要

バッチジョブ `productJob`（および類似のParquetエクスポートジョブ）の実行中に、`exit status 137` というエラーコードでプロセスが終了する問題が発生しました。これはLinux環境において、プロセスがメモリ不足（Out Of Memory, OOM）によって強制終了されたことを示唆しています。

ログには以下のメッセージが確認されました。
`ParquetWriter 'genericParquetExportTaskletWriter' Close called. Total records buffered: 122318.`

このメッセージは、`ParquetWriter` がクローズされる時点で12万件以上のレコードをメモリ上にバッファリングしていたことを示しており、これがOOMの直接的な原因である可能性が高いと判断されました。

## 2. 原因分析

問題の根本原因は、`github.com/xitongsys/parquet-go` ライブラリの `Writer` の挙動と、それをラップする `surfin/pkg/batch/component/step/writer/parquet_writer.go` および `surfin/pkg/batch/component/tasklet/generic/parquet_export_tasklet.go` の連携にありました。

### 2.1. parquet-go ライブラリの挙動

`github.com/xitongsys/parquet-go` の `Writer` は、`Write` メソッドでデータを受け取ると、そのデータを内部バッファに蓄積します。この内部バッファは、`WriteStop`（または `Close`）が呼ばれるまで解放されず、全てのデータがメモリ上に保持されます。`Flush` メソッドを呼び出すことで、内部バッファのデータを強制的にディスク（または指定された `io.Writer`）に書き出し、メモリを解放することができます。

### 2.2. ParquetWriter (surfin) の初期実装

`surfin/pkg/batch/component/step/writer/parquet_writer.go` の `ParquetWriter` は、`Open` メソッドで初期化され、`Write` メソッドでアイテムを受け取ります。しかし、初期の実装では `Write` メソッド内で `parquet-go` の `Writer.Flush()` を呼び出すロジックがありませんでした。全てのデータは `ParquetWriter` の `Close` メソッドが呼ばれるまで `parquet-go` の内部バッファに蓄積され続けました。

### 2.3. GenericParquetExportTasklet の初期実装

`surfin/pkg/batch/component/tasklet/generic/parquet_export_tasklet.go` の `GenericParquetExportTasklet` は、`SqlCursorReader` からデータを読み込み、`t.config.ReadBufferSize`（例: 1000）ごとに `t.parquetWriter.Write(ctx, batch)` を呼び出していました。

この `t.parquetWriter.Write` の呼び出しは、`ParquetWriter` の `Write` メソッドに転送されますが、`ParquetWriter` は内部で `Flush` を行わないため、`parquet-go` の `Writer` は読み込まれた全てのレコードをメモリに保持し続け、最終的にOOMが発生しました。

## 3. 解決策の検討と実装

OOMを解決するためには、`parquet-go` の `Writer` の内部バッファを定期的にフラッシュし、メモリを解放する必要があります。

### 3.1. 最初の試み（不完全な修正）

`ParquetWriter` の `Write` メソッド内に `recordsBuffered` と `flushThreshold` を導入し、`flushThreshold` に達するごとに `pw.Writer.Flush()` を呼び出すロジックを追加しました。

```go
// surfin/pkg/batch/component/step/writer/parquet_writer.go (初期の修正案)
func (pw *ParquetWriter[T]) Write(ctx context.Context, items []T) error {
    // ...
    for _, item := range items {
        if err := pw.Writer.Write(item); err != nil { /* ... */ }
        pw.recordsBuffered++
    }
    if pw.recordsBuffered >= pw.flushThreshold {
        if err := pw.Writer.Flush(); err != nil { /* ... */ }
        pw.recordsBuffered = 0
    }
    return nil
}

```

この修正は、`ParquetWriter` が `Write` を呼び出されるたびに、`batch` のサイズ（最大 `t.config.ReadBufferSize`）だけ `pw.recordsBuffered` を増やし、その直後に `Flush` が実行されることになります。これは、`ParquetWriter` が `Write` を呼び出されるたびに `Flush` を実行するのとほぼ同じ意味になります。

`GenericParquetExportTasklet` の `Writer Goroutine` が `batch` 処理を行っているにもかかわらず、`ParquetWriter` がその `batch` ごとに `Flush` を行うと、`parquet-go` の内部バッファリングの恩恵を十分に受けられず、I/Oオーバーヘッドが増加する可能性がありました。

### 3.2. 最終的な解決策（適切なフラッシュ制御）

`parquet-go` の内部バッファリングを最大限に活用しつつ、メモリ使用量を制御するため、`Flush` のタイミングを `GenericParquetExportTasklet` が制御するように変更しました。

**変更点:**

1. `surfin/pkg/batch/core/application/port/item_writer.go`: `ItemFlusher` インターフェースを新しく定義しました。これにより、`ItemWriter` の実装が `Flush` 機能を提供できることを明示します。
```go
type ItemFlusher interface {
    Flush(ctx context.Context) error
}

```


2. `surfin/pkg/batch/component/step/writer/parquet_writer.go`:
* `ParquetWriter` 構造体から `recordsBuffered` と `flushThreshold` フィールドを削除しました。
* `Write` メソッドから `pw.recordsBuffered` のインクリメントと `if pw.recordsBuffered >= pw.flushThreshold` ブロックを削除しました。
* `ItemFlusher` インターフェースを実装するために、`Flush` メソッドを `ParquetWriter` に追加しました。この `Flush` メソッドは、内部の `parquet-go` の `Writer.Flush()` を呼び出します。
* `Close` メソッドでは、`WriteStop()` を呼び出す前に、残りのバッファを確実にフラッシュするように `Flush()` を呼び出すロジックを維持しました。


3. `surfin/pkg/batch/component/tasklet/generic/parquet_export_tasklet.go`: `Execute` メソッド内の `Writer Goroutine` で、`t.parquetWriter.Write(ctx, batch)` を呼び出した後、明示的に `t.parquetWriter.(port.ItemFlusher).Flush(ctx)` を呼び出すように変更しました。

```go
// surfin/pkg/batch/component/tasklet/generic/parquet_export_tasklet.go (抜粋)
// Writer Goroutine
go func() {
    // ...
    if len(batch) >= t.config.ReadBufferSize {
        if err := t.parquetWriter.Write(ctx, batch); err != nil { /* ... */ }
        batch = make([]T, 0, t.config.ReadBufferSize) // Reset batch

        // Explicitly flush the ParquetWriter after each batch is written.
        if flusher, ok := t.parquetWriter.(port.ItemFlusher); ok {
            if err := flusher.Flush(ctx); err != nil { /* ... */ }
        } else {
            logger.Warnf("ParquetWriter for step '%s' does not implement ItemFlusher. Memory usage might be high.", stepExecution.StepName)
        }
    }
    // ...
}()

```

この最終的な修正により、`ParquetWriter` は `GenericParquetExportTasklet` から明示的に `Flush` が指示されたときにのみ内部バッファを書き出すようになり、`parquet-go` の内部バッファリングを活かしつつ、`t.config.ReadBufferSize` ごとにメモリが解放されるため、メモリ使用量の制御が適切に行われるようになりました。

## 4. 今後の展望

この修正によりOOM問題は解決されるはずですが、大規模なデータ処理においては、以下の点も考慮に入れるとさらに堅牢なシステムを構築できます。

* **ReadBufferSize のチューニング**: `products_job.yaml` の `readBufferSize` は、データベースからの読み込みとParquetへの書き込みのバッチサイズを決定します。この値は、システムのメモリ容量、CPU性能、ネットワーク帯域幅、およびデータ特性に応じて最適化されるべきです。
* **監視とアラート**: メモリ使用量、CPU使用率、I/Oスループットなどのメトリクスを継続的に監視し、異常を検知した際にアラートを発する仕組みを導入することで、将来的なパフォーマンス問題やリソース枯渇を早期に発見できます。
* **エラーハンドリングの強化**: `errChan` を使用したエラー伝播は基本的なものですが、より複雑なジョブでは、エラーの種類に応じたリトライ戦略や、デッドレターキューへの書き込みなどの高度なエラーハンドリングを検討することも有効です。
* **Parquetファイルサイズの最適化**: Parquetファイルのサイズは、読み込み性能に影響を与えます。あまりに小さなファイルが多数生成される「Small Files Problem」を避けるため、必要に応じてパーティション戦略やバッチサイズを調整し、適切なファイルサイズになるように制御することが望ましいです。
