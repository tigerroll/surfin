# チェックポイントおよび永続化戦略 (Checkpointing and Persistence Strategy)

## 1. 目的
* 本フレームワークにおけるチェックポイント戦略は、バッチ処理の信頼性向上と、JSR-352 (Jakarta Batch) 仕様への準拠を目的とする。
* ジョブの再開（Restart）を可能にするため、ステップ実行中の状態（`ExecutionContext`）を確実に永続化する。

## 2. ライフサイクルと責任分界点
チェックポイントの永続化は、以下の3つのライフサイクルメソッドを通じて管理される。

| メソッド | 役割 | 責任者 |
| :--- | :--- | :--- |
| `Open` | 状態の復元（DB → メモリ） | コンテナ (ChunkStep) |
| `Update` | 状態の同期（コンポーネント → ExecutionContext） | コンポーネント (Reader/Writer) |
| `Close` | 最終状態の永続化（ExecutionContext → DB） | コンテナ (ChunkStep) |

### 2.1. コンポーネントの責任 (`Update`)
`ItemStream` インターフェースを実装するコンポーネントは、`Update` メソッド内で自身の現在の処理位置や統計情報を `ExecutionContext` に書き込む責任を持つ。
*   **注意:** `Update` メソッド自体はメモリ上の `ExecutionContext` を更新するのみであり、DBへの書き込みは行わない。

### 2.2. コンテナの責任 (`saveCheckpoint`)
`ChunkStep` などのコンテナは、以下のタイミングで `ExecutionContext` をリポジトリ（DB）へ永続化する責任を持つ。
1.  **チャンクコミット後:** トランザクション成功直後に `saveCheckpoint` を呼び出す。
2.  **ステップ終了時:** `Close` メソッド内で必ず `saveCheckpoint` を呼び出し、最終状態を保存する。

## 3. JSR-352 準拠の設計指針
本フレームワークは JSR-352 の設計思想に基づき、以下のルールを適用する。

1.  **アトミックな保存:** チェックポイントの保存は、チャンクのトランザクションコミットと論理的に連動させる。
2.  **最終保存の保証:** ステップが正常終了、異常終了、キャンセルされた場合であっても、`Close` メソッドを通じて最終的な状態を保存する。
3.  **冪等性:** 再開時に同じデータが二重処理されないよう、`Open` メソッドで `ExecutionContext` から読み込んだオフセットを正しく適用する。

## 4. 実装ルール
*   **`Close` メソッドの義務:** 全ての `Step` 実装は、`Close` メソッドの終了直前に `saveCheckpoint` を呼び出さなければならない。
*   **エラーハンドリング:** `saveCheckpoint` が失敗した場合、そのエラーは無視せず、ステップの失敗として扱うこと。
*   **ログ出力:** 永続化のタイミングと、保存される `ExecutionContext` の中身は `DEBUG` レベルでログ出力し、トレース可能にすること。

## 5. 監視と検証
チェックポイントが正しく機能しているかは、以下のクエリで検証する。

```sql
-- ステップ実行ごとのコンテキスト確認
-- 現在の設計では、ExecutionContextは batch_step_execution テーブルの execution_context カラムに保存されます。
SELECT id, step_name, execution_context FROM batch_metadata.batch_step_execution;
```

## 6. Design Contract

チェックポイント機構を実装する際、コンポーネントとコンテナ（ChunkStep）は以下の契約を遵守しなければならない。

### 6.1. Component Contract (ItemStream)
*   **Idempotency:** `Update` メソッドは、複数回呼び出されても副作用が累積しないように実装すること。
*   **Memory-Only:** `Update` メソッド内で外部リソース（DB等）への書き込みを行ってはならない。
*   **Namespace:** `ExecutionContext` に保存するキーは、コンポーネント名（例: `reader.offset`）をプレフィックスとして使用し、衝突を避けること。

### 6.2. Container Contract (ChunkStep)
*   **Persistence Responsibility:** `saveCheckpoint` は、トランザクションコミット直後に呼び出し、永続化の成否を監視すること。
*   **Error Propagation:** `saveCheckpoint` が失敗した場合、そのエラーは握りつぶさず、ステップの失敗として伝播させ、ジョブを `FAILED` 状態に遷移させること。
*   **Finalization:** `Close` メソッドは、正常終了・異常終了に関わらず必ず呼び出され、最終的なチェックポイントを保存すること。
