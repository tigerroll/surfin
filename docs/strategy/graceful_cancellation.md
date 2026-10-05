# Graceful Cancellation Semantics

## 1. 目的
バッチ処理実行中にキャンセル要求（`context.Context` のキャンセル）が発生した際、システムがどのような状態遷移を行い、再開時にどのような挙動をとるべきかという「実行意味論（Execution Semantics）」を定義する。

## 2. 基本方針
Surfin における Graceful Cancellation は、以下の原則を目標とする。

1.  **整合性の優先**: 処理中のトランザクションはロールバックし、データ不整合を防ぐ。
2.  **再開可能性の維持**: 最後に確定した Restart Checkpoint を基準として、再開時の処理結果が予測可能であること。
3.  **即時性の尊重**: キャンセル要求を受け取った場合、可能な限り速やかに新しい処理の開始を停止する。

## 3. 実行意味論と Failure Matrix
キャンセル発生タイミングと、その後の状態（Commit, Checkpoint, Restart）の関係を以下のように定義する。

| Case | 状況 | Commit | Checkpoint | Restart 挙動 |
| :--- | :--- | :--- | :--- | :--- |
| **A** | Chunk 実行前 | なし | 維持 | Checkpoint から再開 |
| **B** | Chunk 実行中 | Rollback | 維持 | Checkpoint から再開 |
| **C** | Chunk Commit 直後 | 確定 | 未保存 | **要定義** (重複実行の可能性あり) |
| **D** | Checkpoint 保存中 | 確定 | 不定 | **要定義** (原子性保証の課題) |

### 課題: 原子性の欠如
現在のアーキテクチャでは、Workload DB の Commit と Metadata DB の Checkpoint 保存は別々のトランザクションである。そのため、Case C および D において「Commit は成功したが Checkpoint 保存に失敗した」という状態が発生しうる。
この場合、再開時に「既に Commit 済みの Chunk」が再実行されるリスクがある。この重複実行を許容するか、あるいは原子性を保証する仕組み（例: 2相コミットや冪等性の強制）を導入するかは、今後の設計課題とする。

## 4. 状態遷移モデル
キャンセル発生時の Worker と Controller の状態遷移は以下のように定義する。

*   **Worker**: `context.Canceled` を検知した場合、`BatchStatusCancelled` へ遷移する。
*   **Controller**: Worker の `CANCELLED` を検知した場合、集約ロジックにより `BatchStatusStopped` へ遷移する。

### 集約優先度 (Failure Matrix)
複数の Worker が混在する場合、以下の優先度で Controller の最終状態を決定する。
`FAILED` > `STOPPED` / `CANCELLED` > `COMPLETED`

## 5. 実装上の注意点
*   **Context の伝播**: `ItemReader`, `ItemWriter` 等の長時間ブロックする処理は `ctx.Done()` を監視すること。
*   **トランザクション管理**: キャンセル時は `currentTxManager.Rollback()` を呼び出し、リソースを解放すること。
*   **Restartability**: `JobOperator.Restart` は `BatchStatusCancelled` を再開可能な状態として扱う。

## 6. 実行意味論の確立 (Partition Execution)
Partition Execution においては、Worker の状態遷移と Controller の集約ロジックを Failure Matrix として定義し、実装レベルでこれを保証する。

| Worker State | Controller State | 備考 |
| :--- | :--- | :--- |
| `COMPLETE` | `COMPLETE` | 正常終了 |
| `FAILED` | `FAILED` | 業務/システムエラー |
| `STOPPED` | `STOPPED` | 明示的な停止 |
| `CANCELLED` | `STOPPED` | 上位キャンセル伝播 |
| 混在 (FAILED/...) | `FAILED` | 優先度最高 |
| 混在 (CANCELLED/...) | `STOPPED` | 優先度中 |

この意味論により、Partition 単位の並行実行においても、一貫した状態遷移と再開可能性を保証する。
