# 技術設計ドキュメント：大規模バッチ処理の最適化戦略

**〜JSR-352 の規律とGo言語の機動力の融合〜**

## 1. 概要

本ドキュメントは、1億件を超える大規模データを扱う決済システム等のミッションクリティカルなバッチ処理において、Go言語製フレームワーク**Surfin**を最大限に活用するための設計指針を定義する。

Java の JSR-352 が持つ構造化された「規律」を維持しつつ、Goの言語特性である並行処理（Goroutine/Channel）を「ハイブリッド・パイプライン Tasklet」という形でカプセル化することで、リソース効率と実行速度の極大化を実現する。

---

## 2. 汎用コンポーネント：SqlCursorReader

Javaの `JdbcCursorItemReader` の思想を継承し、Goの `database/sql` に最適化したReader。

### 2.1 設計の背景

Goにおいて、`db.Query()` が返す `*sql.Rows` は本質的にイテレータ（Cursor）である。これをSurfinのライフサイクルに適合させることで、巨大なデータセットをメモリに載せることなく、1件ずつ安全にフェッチする。

### 2.2 実装のポイント

* **Genericsの活用:** `SqlCursorReader[T any]` として定義し、任意の構造体にマッピング可能とする。
* **ライフサイクル管理:** Surfinの `Open/Read/Close` メソッドに合わせ、DB接続の維持と確実な解放を行う。

```go
// SqlCursorReader の実装イメージ
type SqlCursorReader[T any] struct {
    db     *sql.DB
    query  string
    args   []any
    mapper func(*sql.Rows) (T, error)
    rows   *sql.Rows
}

func (r *SqlCursorReader[T]) Open() error {
    rows, err := r.db.Query(r.query, r.args...)
    if err != nil {
        return err
    }
    r.rows = rows
    return nil
}

func (r *SqlCursorReader[T]) Read() (T, error) {
    if r.rows.Next() {
        return r.mapper(r.rows)
    }
    if err := r.rows.Err(); err != nil {
        return *new(T), err
    }
    return *new(T), io.EOF // データ終了
}

func (r *SqlCursorReader[T]) Close() error {
    if r.rows != nil {
        return r.rows.Close()
    }
    return nil
}

```

---

## 3. 核心：ハイブリッド・パイプライン Tasklet

Goの強みを最大限に活かし、複雑な並行処理をカプセル化する**ファサード（Facade）**としての設計。

### 3.1 構造：Tasklet内パイプライン

従来のTaskletは逐次処理が基本だが、本設計ではTasklet内部に「Reader/WriterのGoroutine」を生成し、Channelで繋ぐ並列パイプライン構造を採用する。

1. **Reader Goroutine:** `SqlCursorReader` を使用し、DBからデータを取得してChannelに供給。
2. **Channel:** データをバッファリングし、I/O待ちとCPU処理の速度差を吸収。
3. **Writer Goroutine:** Channelからデータを受け取り、**10,000件単位**でParquet形式等のバッファに集約・出力。

### 3.2 設計の正当性

* **I/OとCPUの並列化:** DB読み込みとParquet圧縮（CPU負荷）を別スレッドで回すことで、1億件クラスの処理時間を劇的に短縮する。
* **Facadeによる隠蔽:** 開発者は「読み書き」のロジックに集中でき、複雑な `errgroup` や `context` による連鎖停止ロジックを意識する必要がない。
* **進捗の可視化:** 内部の進捗をSurfinの `StepContribution` に逐次反映し、フレームワークの管理下に置く。

---

## 4. データ出力戦略：10,000件単位のParquet変換

ミッションクリティカルな分析基盤（AWS Athena等）との相性を考慮し、出力はParquetを推奨する。

* **グループ書き出し:** Parquetは列指向フォーマットであるため、10,000件程度を一つの「Row Group」としてメモリ上で構築し、一気にフラッシュすることで圧縮効率とI/O効率を最大化する。
* **Taskletとの相性:** 1件ずつのChunk処理よりも、Tasklet内でバッファを制御する方がParquetライブラリのAPIを素直に活用できる。

---

## 5. 信頼性の担保（ミッションクリティカル要件）

| 機能 | 内容 |
| --- | --- |
| **再開性 (Restartability)** | 異常終了時、保持された `ReadCount` を元に、SQLの `OFFSET` 等で途中から再開。 |
| **エラー伝搬** | `errgroup` を活用。パイプラインのどこかでエラーが起きれば即座に全停止し、ステップ失敗として記録。 |
| **型安全性** | 全てのデータパスにおいてGenericsを徹底し、Goの静的解析でランタイムエラーを未然に防ぐ。 |

---

## 6. 結論

本設計は、 JSR-352 が培ってきた「エンタープライズの規律」をリスペクトしつつ、Go言語にしかできない「爆速の内部パイプライン」を実装するものである。

**「外側は堅牢なJSR-352、中身はGoのGoroutineエンジン」**

このハイブリッドな構造こそが、JavaからGoへの移行を検討している開発チームにとっての「正解」であり、ミッションクリティカルなバッチ基盤としてのSurfinの真価を証明する。

---

### 付録：基底 PipelineTasklet テンプレート

```go
type PipelineTasklet[T any] struct {
    Reader     func(ctx context.Context, dataCh chan<- T) error
    Writer     func(ctx context.Context, dataCh <-chan T, report func(int64)) error
    BufferSize int
}

func (t *PipelineTasklet[T]) Execute(contribution *surfin.StepContribution, chunkContext *surfin.ChunkContext) error {
    g, ctx := errgroup.WithContext(context.Background())
    dataCh := make(chan T, t.BufferSize)

    g.Go(func() error {
        defer close(dataCh)
        return t.Reader(ctx, dataCh)
    })

    g.Go(func() error {
        return t.Writer(ctx, dataCh, func(n int64) {
            contribution.IncrementReadCount(int(n))
        })
    })

    return g.Wait()
}

```

