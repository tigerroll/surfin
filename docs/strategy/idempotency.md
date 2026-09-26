# 冪等性および重複処理防止戦略 (Idempotency and Duplicate Processing Strategy)

## 1. 目的
バッチ処理における再実行（Restart）やリトライ（Retry）発生時に、データが二重に処理・保存されることを防ぎ、システム全体の整合性を保つことを目的とする。

## 2. 基本原則
Surfin Batch Frameworkでは、以下の原則に基づき冪等性を担保する。

1.  **コンポーネントの責任:** `ItemWriter` は、自身の書き込み処理が冪等であることを保証しなければならない。
2.  **フレームワークの支援:** `ExecutionContext` を通じて処理済み状態を永続化し、再実行時に読み込めるようにする。
3.  **明示的な契約:** 冪等性を担保するコンポーネントは、フレームワークが提供するインターフェースを通じてその旨を明示する。

## 3. 冪等性担保パターン
コンポーネント開発者は、以下のいずれかのパターンを選択して実装すること。

### 3.1. Upsert (Merge) パターン
データベースへの書き込みにおいて、主キーやユニークキーを用いて「存在すれば更新、存在しなければ挿入」を行う。
*   **適用例:** 顧客マスタの同期、集計データの更新。
*   **実装:** SQLの `INSERT ... ON DUPLICATE KEY UPDATE` や `UPSERT` 構文を利用する。

### 3.2. Optimistic Locking パターン
バージョンカラムやタイムスタンプを用いて、古いデータによる上書きを防ぐ。
*   **適用例:** 複数のジョブが同一レコードを更新する可能性がある場合。
*   **実装:** `UPDATE ... WHERE id = ? AND version = ?` を実行し、更新件数が0件の場合はエラーとして扱う。

### 3.3. State Tracking パターン (ExecutionContext)
`ExecutionContext` を利用して、処理済みIDやオフセットを追跡する。
*   **適用例:** 外部APIへの書き込みなど、DBトランザクションが使えない場合。
*   **実装:** `ItemStream.Update` で処理済みIDを保存し、`ItemStream.Open` で再開位置を復元する。

## 4. フレームワークによる支援 (Framework Support)

### 4.1. IdempotentWriter インターフェース
冪等性を担保するWriterであることを明示するために、以下のインターフェースを導入する。

```go
// IdempotentWriter は、書き込み処理が冪等であることを保証するWriterのためのインターフェースです。
type IdempotentWriter[I any] interface {
    ItemWriter[I]
    // IsIdempotent は、このWriterが冪等性を担保しているかを返します。
    IsIdempotent() bool
}
```

フレームワークは、このインターフェースを実装しているコンポーネントに対して、再実行時の挙動を最適化する（例: 警告ログの抑制など）ことが可能となる。

## 5. 検証戦略 (Verification Strategy)
冪等性はドキュメントだけでなく、テストによって保証されなければならない。

*   **Failure Matrix Test:** `test/semantics/failure_matrix_test.go` を拡張し、「コミット成功直後のチェックポイント保存失敗」や「部分的な書き込み失敗」をシミュレートし、再実行時にデータが重複しないことを確認すること。
*   **Restart Integration Test:** `test/pkg/batch/engine/step/item/chunk_step_test.go` のような統合テストにおいて、意図的にジョブを中断・再開させ、最終的なデータ件数が期待値と一致することを確認すること。

## 6. 開発者への契約 (Contract)

*   **Writer:** 冪等性を担保できないWriterは、その旨をドキュメントに明記し、再実行時の挙動（例: 「再実行不可」）を定義すること。
*   **Processor:** 状態を持つProcessor（ステートフルProcessor）は、必ず `ItemStream` インターフェースを実装し、状態の保存・復元をサポートすること。
*   **ExecutionContext:** キーの衝突を避けるため、`component.name.key` のような階層構造（`PutNested`）を使用すること。
