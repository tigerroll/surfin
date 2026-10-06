# Partition Execution Design

## 1. 概念定義
Partition Execution における主要なコンポーネントを以下のように定義する。

- **Partition**: 処理範囲を持つ論理的な実行単位。実行場所や実行方式を規定せず、Worker によって Local または将来の Remote Execution で実行される。
- **Worker**: Partition を担当して実行する論理的な実行主体。現在の Local Execution では goroutine として実装される。
- **Controller**: Partition の割り当て、Worker の実行管理、および結果集約を行う。
- **concurrency**: 同時に実行する Partition の最大数。

> **注記**: Remote Execution は将来の実行方式として検討対象であるが、v0.1.0 の実装範囲には含まれない。

## 2. 設計原則
- **Go の concurrency model を利用する**: 独自の Executor 抽象化は導入せず、`goroutine`, `context`, `errgroup`, `semaphore` を組み合わせる。
- **実行方式を型で分けない**: `concurrency` というパラメータで実行数を制御し、実行方式ごとに型を作らない。
- **Execution Semantics を優先する**: 並行実行そのものよりも、「Partition がどのように実行され、失敗したときに何が起きるか」を明確にする。

## 3. 実行モデル
Partition Execution では、Worker ごとに独立した実行結果が発生する。各 Partition は独立した Worker によって実行され、その実行結果は独立した `StepExecution` として管理される。

```text
[STARTING] -> [STARTED] -> [COMPLETED]
                 |    |
                 |    +-> [FAILED]
                 |    +-> [STOPPED]
                 +-> [CANCELLED]
```

| 状態 | 定義 | 典型的な原因 |
| :--- | :--- | :--- |
| **COMPLETED** | 正常終了 | 処理の全完了 |
| **FAILED** | エラーによる終了 | 業務エラー / システムエラー |
| **STOPPED** | 明示的な停止 | ユーザーまたはControllerからのStop要求 |
| **CANCELLED** | 上位キャンセルによる終了 | 親Contextのキャンセル伝播 |

## 4. Concurrency Model
`concurrency` は、同時に実行される Partition の最大数を表す。

- **Partition 数と concurrency は別の概念**:
  - `concurrency` は goroutine の生成数ではなく、実際に処理を実行している Partition の上限を表す。
- **Local Execution**:
  - Partition ごとに goroutine を起動し、`semaphore` を用いて「実行中 Partition 数」を制御する。
  - 現在の Local Execution では、各 Partition を Worker が担当するため、結果として同時実行される Worker 数も `concurrency` 以下となる。これにより、リソース（DB Connection, メモリ等）の過剰な消費を防止する。

## 5. Cancellation Mechanism
キャンセル要求は `context.Context` を通じて伝播される。

- **Worker**: `context.Canceled` を検知した場合、`BatchStatusCancelled` へ遷移する。
- **Controller**: Worker の `CANCELLED` を検知した場合、集約ロジックにより `BatchStatusStopped` へ遷移する。
- **STOPPED と CANCELLED の区別**:
    - **Execution State**: `CANCELLED` は親 Context からのキャンセル伝播によって終了したことを示す独立した状態。
    - **ExitStatus**: 既存の ExitStatus 体系との互換性のため、`CANCELLED` 状態の Worker は `STOPPED` として報告される。これにより、内部的には区別しつつ、外部には一貫した終了ステータスを提供する。

## 6. Failure Matrix (集約ロジック)
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
