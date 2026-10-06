# Graceful Cancellation Semantics

## 1. 目的
バッチ処理実行中にキャンセル要求（`context.Context` のキャンセル）が発生した際、システムがどのような状態遷移を行い、再開時にどのような挙動をとるべきかという「実行意味論（Execution Semantics）」を定義する。

## 2. 概念定義
- **Partition**: 処理範囲を持つ論理的な実行単位。実行場所や実行方式を規定せず、Worker によって Local または将来の Remote Execution で実行される。
- **Worker**: Partition を担当して実行する論理的な実行主体。現在の Local Execution では goroutine として実装される。
- **Controller**: Partition の割り当て、Worker の実行管理、および結果集約を行う。
- **concurrency**: 同時に実行する Partition の最大数。

> **注記**: Remote Execution は将来の実行方式として検討対象であるが、v0.1.0 の実装範囲には含まれない。

## 3. Graceful Cancellation における目標原則
Surfin における Graceful Cancellation は、以下の原則を目標とする。

1.  **整合性の優先**: 処理中のトランザクションはロールバックし、データ不整合を防ぐ。
2.  **再開可能性の維持**: 最後に正常に Commit された処理位置を Restart Checkpoint として維持する。未 Commit の処理を Checkpoint として確定させない。
3.  **即時性の尊重**: キャンセル要求を受け取った場合、可能な限り速やかに新しい処理の開始を停止する。

## 4. Graceful Cancellation Semantics (将来の目標)
Worker は `context.Done()` を受信した際、即座に終了するのではなく、以下の手順で「Graceful」に終了することを目標とする。

1. **新しい処理の開始禁止**: 新しい Chunk や Tasklet の実行を開始しない。
2. **現在の処理の完了**: 現在実行中の Chunk や Transaction をコミットまたはロールバックする。
3. **Checkpoint の維持**: 最後に正常に Commit された処理位置を Restart Checkpoint として維持する。
   Checkpoint の保存自体が Commit と原子的でないため、Cancellation に伴って新しい Checkpoint が確定できない場合は、
   直前の Checkpoint からの再実行を許容する。
4. **状態遷移**: 最終的に `CANCELLED` 状態へ遷移する。

## 5. Cancellation Mechanism
キャンセル要求は `context.Context` を通じて伝播される。

- **Worker**: `context.Canceled` を検知した場合、`BatchStatusCancelled` へ遷移する。
- **Controller**: Worker の `CANCELLED` を検知した場合、集約ロジックにより `BatchStatusStopped` へ遷移する。
- **CANCELLED のスコープ**: `CANCELLED` は Worker/Step-level の Execution State であり、Controller/Job-level では `STOPPED` に集約される。

## 6. 将来の Graceful Cancellation における実行意味論と Failure Matrix
以下は、Graceful Cancellation を実装した場合に成立させるべき Commit、Checkpoint、Restart の意味論を定義する。

| Case | 状況 | Commit | Checkpoint | Restart 挙動 |
| :--- | :--- | :--- | :--- | :--- |
| **A** | Chunk 実行前 | なし | 維持 | Checkpoint から再開 |
| **B** | Chunk 実行中 | Rollback | 維持 | Checkpoint から再開 |
| **C** | Chunk Commit 直後 | 確定 | 未保存 | 再実行の可能性あり |
| **D** | Checkpoint 保存中 | 確定 | 不定 | 再実行の可能性あり |

### Commit と Checkpoint の非原子性
Workload DB の Commit と Metadata DB の Checkpoint 保存は、別々のトランザクションとして扱う。Surfin では両者を 2PC によって原子的に扱うことはしない。そのため、Commit 後に Checkpoint が保存されなかった場合、Restart によって直前の Chunk が再実行される可能性がある。この再実行に対しては、Restart Semantics、冪等性、Sage Pattern / Compensation などによって処理結果の整合性を維持することを基本方針とする。

## 7. 状態遷移モデル
### 集約優先度 (Failure Matrix)
複数の Worker が混在する場合、以下の優先度で Controller の最終状態を決定する。
`FAILED` > `STOPPED` > `COMPLETED`

| Worker State | Controller State | 備考 |
| :--- | :--- | :--- |
| `COMPLETED` | `COMPLETED` | 正常終了 |
| `FAILED` | `FAILED` | 業務/システムエラー |
| `STOPPED` | `STOPPED` | 明示的な停止 |
| `CANCELLED` | `STOPPED` | 上位キャンセル伝播 (Controller で STOPPED に集約) |
| `ABANDONED` | `FAILED` | 破棄された Worker は Controller Failure として扱う |
| 混在 (FAILED/...) | `FAILED` | 優先度最高 |
| 混在 (CANCELLED/...) | `STOPPED` | 優先度中 |

## 8. 実装上の注意点
*   **Context の伝播**: `ItemReader`, `ItemWriter` 等の長時間ブロックする処理は `ctx.Done()` を監視すること。
*   **トランザクション管理**: キャンセル時は `currentTxManager.Rollback()` を呼び出し、リソースを解放すること。
*   **Restartability**: `CANCELLED` は Worker/Step-level の Execution State として扱われる。Controller/Job-level では `STOPPED` に集約されるため、Restart 時には Controller/Job の状態に基づいて再開可能性を判定する。
