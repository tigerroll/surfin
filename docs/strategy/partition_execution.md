# Partition Execution Strategy

## 1. 概念定義
Partition Execution における主要なコンポーネントを以下のように定義する。

- **Partition**: 処理範囲を持つ論理的な実行単位。実行場所や実行方式を規定せず、Worker によって Local または将来の Remote Execution で実行される。
- **Worker**: Partition を担当して実行する論理的な実行主体。現在の Local Execution では goroutine として実装される。
- **Controller**: Partition の割り当て、Worker の実行管理、および結果集約を行う。
- **concurrency**: 同時に実行する Partition の最大数。

> **注記**: Remote Execution は将来の実行方式として検討対象であるが、v0.1.0 の実装範囲には含まれない。

## 2. 戦略方針
Partition Execution は、処理範囲を複数の Partition に分割し、それぞれを独立した実行単位として処理するための基盤である。Controller は Worker のライフサイクルを管理し、各 Worker の実行結果を統合して最終的な StepExecution の状態を決定する。

## 3. Concurrency Model
`concurrency` は、同時に実行される Partition の最大数を表す。

- **Partition 数と concurrency は別の概念**:
  - `concurrency` は goroutine の生成数ではなく、実際に処理を実行している Partition の上限を表す。
- **Local Execution**:
  - Partition ごとに goroutine を起動し、`semaphore` を用いて「実行中 Partition 数」を制御する。<br/>現在の Local Execution では、各 Partition を Worker が担当するため、結果として同時実行される Worker 数も `concurrency` 以下となる。<br/>これにより、リソース（DB Connection, メモリ等）の過剰な消費を防止する。

## 4. Cancellation Mechanism
キャンセル要求は `context.Context` を通じて伝播される。

- **Worker**: `context.Canceled` を検知した場合、`BatchStatusCancelled` へ遷移する。
- **Controller**: Worker の `CANCELLED` を検知した場合、集約ロジックにより `BatchStatusStopped` へ遷移する。
- **CANCELLED のスコープ**: `CANCELLED` は Worker/Step-level の Execution State であり、Controller/Job-level では `STOPPED` に集約される。

## 5. Failure Matrix (集約ロジック)
Controller は以下の優先度で最終状態を決定する。

| Case | Worker State | Controller State |
| :--- | :--- | :--- |
| Worker 自身が成功 | `COMPLETED` | `COMPLETED` |
| Worker Failure | `FAILED` | `FAILED` |
| Controller Stop | `STOPPED` | `STOPPED` |
| Context Cancel | `CANCELLED` | `STOPPED` |
| Worker 混在 (FAILED/CANCELLED/COMPLETED) | - | `FAILED` |
| Worker 混在 (CANCELLED/COMPLETED) | - | `STOPPED` |

**Failure Matrix は単なるテストケース一覧ではなく、Partition Execution Semantics の executable specification として扱う。**
