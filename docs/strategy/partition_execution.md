# Partition Execution

## 1. 目的

Surfin の `PartitionStep` は、1つの Step を複数の Partition に分割し、それぞれを独立した Worker として実行する。

現在の実装では、Partition ごとの Worker を goroutine で起動することで、Partition 単位の並行実行を実現している。

Partition Execution を強化するにあたり、以下を目的とする。

* Partition の並行実行数を制御する
* `PartitionStep` ごとに並行実行数を設定できるようにする
* Partition 単位の失敗・キャンセル・再実行を明確にする
* Partition の実行結果を Controller Step に正しく集約する
* 大量の Partition による DB、メモリ、外部 API 等への過剰な負荷を防止する
* 将来的な Remote Execution への拡張余地を残す
* Go の concurrency primitive を活用し、不要な Executor 抽象を導入しない

Surfin は JSR-352 / Spring Batch の Partitioning の考え方を参考にする一方で、その実装まで Java/Spring の設計パターンに合わせることは目的としない。

**実行意味論は明確にし、実装は Go らしく単純にする。**

---

## 2. 現在の実装

現在の `PartitionStep` は、Partition ごとに goroutine を起動して Worker Step を実行する。

概念的には、

```text
PartitionStep
     │
     ├── Partition A ── goroutine
     ├── Partition B ── goroutine
     ├── Partition C ── goroutine
     └── ...
```

という構造になっている。

現時点では、Partition 数に対する**明示的な並行実行数の上限は設けていない**。

そのため、Partition が100個存在すれば、最大100個の Worker が並行して実行される可能性がある。

goroutine 自体は軽量だが、Worker が使用する外部リソースまで無制限に利用できるわけではない。

例えば、

* DB Connection
* メモリ
* CPU
* 外部 API
* downstream system

などへの負荷が Partition 数に応じて増大する可能性がある。

そのため、Partition Execution には**並行実行数の制御**が必要になる。

---

## 3. Step 単位の Concurrency

Partition の並行実行数は、Job 全体ではなく **`PartitionStep` ごとに設定できる**ものとする。

例えば、

```yaml
steps:
  - name: import
    type: partition
    partition:
      concurrency: 8

  - name: export
    type: partition
    partition:
      concurrency: 2
```

とする。

これにより、

```text
Job
 │
 ├── import
 │     └── PartitionStep
 │           └── concurrency: 8
 │
 └── export
       └── PartitionStep
             └── concurrency: 2
```

のように、Step ごとに異なる並行実行数を設定できる。

これは Step ごとに処理内容や利用する外部リソースが異なることを考慮した設計である。

---

## 4. Concurrency の意味

`concurrency` は、**同時に実行される Partition Worker の最大数**を表す。

ここで重要なのは、

> **Partition 数と concurrency は別の概念である**

ということである。

`concurrency` は goroutine の生成数ではなく、Partition Worker が実際に処理を実行している数の上限を表す。

現在の実装では、Partition はすべて生成され、それぞれに対応する goroutine が起動される。`concurrency` は、その中で**実際に処理を実行する Worker の数**を `semaphore` によって制御する。

`concurrency` は goroutine の生成数ではなく、Partition Worker が実際に処理を実行している数の上限を表す。

```text
100 Partitions (100 goroutines)
       │
       ▼
┌───────────────────────┐
│ concurrency = 8       │ (semaphore で制御)
└───────────────────────┘
       │
       ├── Worker 1 (Active)
       ├── Worker 2 (Active)
       ...
       ├── Worker 8 (Active)
       │
       └── Worker 9-100 (Waiting)
```

`concurrency` は「実行中 Worker 数」の制御であり、Partition の生成数や待機 goroutine 数そのものを制限するものではない。

---

## 5. Go の concurrency model

Partition Execution は、Serial / Parallel といった実行方式を Executor の型として分けるのではなく、**一つの Partition Execution model として扱う**。

実行数の違いは `concurrency` というパラメータによって表現する。

```text
PartitionStep
     │
     ▼
Partition Generation
     │
     ▼
Partition Execution
     │
     ├── goroutine (全 Partition 分)
     ├── context.Context
     ├── errgroup
     └── semaphore (concurrency 制御)
```

例えば、

```yaml
partition:
  concurrency: 8
```

であれば、最大8個の Partition Worker が並行して実行される。

```yaml
partition:
  concurrency: 1
```

であれば、Partition は1つずつ実行される。

`concurrency: 1` を特別な実行方式として扱う必要はない。

**単一の concurrency パラメータによって実行数を制御する。**

---

## 6. Go の concurrency primitive

Partition Execution では、Go が提供する concurrency primitive を組み合わせて実装する。

| Primitive         | 役割                          |
| ----------------- | --------------------------- |
| goroutine         | Partition Worker の並行実行      |
| `context.Context` | cancellation / deadline の伝播 |
| `errgroup`        | goroutine のライフサイクルとエラー集約    |
| semaphore         | Partition の最大並行実行数の制御       |

重要なのは、独自の Executor abstraction を作るのではなく、Go の concurrency model をそのまま活用することである。

---

## 7. Partition Generation と Execution

`PartitionStep` の責務は、大きく以下に分ける。

```text
PartitionStep
     │
     ├── Partition Generation
     │       └── partitions を生成
     │
     └── Partition Execution
             ├── Worker Step を実行
             ├── concurrency を制御
             ├── cancellation を伝播
             └── 結果を集約
```

Partition の生成方法と、生成された Partition をどのように実行するかは異なる責務として扱う。

ただし、この分離のために、

```text
PartitionExecutor
├── SerialPartitionExecutor
├── ParallelPartitionExecutor
└── RemotePartitionExecutor
```

のような Executor 階層を導入することはしない。

**実行方式を型で切り替えるのではなく、`PartitionStep` の `concurrency` によって実行数を制御する。**

---

## 8. Partition Failure Semantics

Partition Execution では、Worker ごとに独立した実行結果が発生する。

### 8.1. Partition Worker の状態遷移
Worker（各パーティション）は、独立した `StepExecution` として扱われる。

```text
[STARTING] -> [STARTED] -> [COMPLETE]
                 |    |
                 |    +-> [FAILED]
                 |    +-> [STOPPED]
                 +------> [CANCELLED]
```

| 状態 | 定義 | 典型的な原因 |
| :--- | :--- | :--- |
| **COMPLETE** | 正常終了 | 処理の全完了 |
| **FAILED** | エラーによる終了 | 業務エラー / システムエラー |
| **STOPPED** | 明示的な停止 | ユーザーまたはControllerからのStop要求 |
| **CANCELLED** | 上位キャンセルによる終了 | 親Contextのキャンセル伝播 |

---

## 9. Cancellation (キャンセル)

### 9.1. Cancellation のメカニズム
Surfin における Cancellation は、`context.Context` を通じて上位の実行単位（Controller）から Worker へ停止信号を伝播させる仕組みである。

### 9.2. Graceful Cancellation Semantics
Worker は `context.Done()` を受信した際、即座に終了するのではなく、以下の手順で「Graceful」に終了しなければならない。

1. **新しい処理の開始禁止**: 新しい Chunk や Tasklet の実行を開始しない。
2. **現在の処理の完了**: 現在実行中の Chunk や Transaction をコミットまたはロールバックする。
3. **Checkpoint の保存**: 可能な限り現在の状態を保存する。
4. **状態遷移**: 最終的に `CANCELLED` 状態へ遷移する。

この手順により、再実行（Restart）時にデータの一貫性が保たれる。

### 9.3. STOPPED と CANCELLED の区別
状態名から原因を読み取れるよう、以下の通り区別する。

| 状態 | 意味 | 典型的な原因 |
| :--- | :--- | :--- |
| **STOPPED** | 明示的停止 | User / Controller による Stop 要求 |
| **CANCELLED** | 上位キャンセルによる終了 | 親 Context の cancellation 伝播 |

---

## 10. Restart / Checkpoint

Partition Execution では、各 Partition が独立した `StepExecution` と ExecutionContext を持つ。

Restart 時には、正常終了した Partition と失敗した Partition を区別できる必要がある。

この仕様は Partition Failure Matrix として明文化する。

---

## 11. Failure Matrix

Partition Execution についても、通常の ChunkStep と同様に Failure Matrix を仕様として持つ。

| Case | Worker State | Controller State |
| :--- | :--- | :--- |
| Worker 自身が成功 | `COMPLETE` | `COMPLETE` |
| Worker Failure | `FAILED` | `FAILED` |
| Controller Stop | `STOPPED` | `STOPPED` |
| Context Cancel | `CANCELLED` | `CANCELLED` |
| Worker A Failure → B/C cancel | A=`FAILED`, B/C=`CANCELLED` | `FAILED` |
| Cancel中にChunk Commit済み | `CANCELLED` | `CANCELLED` |
| Cancel中にTransaction未Commit | `CANCELLED` | `CANCELLED` |
| Cancel後Restart | `CANCELLED` → 再実行可能 | Job再実行 |

**Failure Matrix は単なるテストケース一覧ではなく、Partition Execution Semantics の executable specification として扱う。**

---

## 12. Remote Execution

将来的には Partition を別の実行基盤へ委譲する Remote Execution を検討する。

ただし、Remote Execution は Local Execution とは異なる性質を持つ。

そのため、現時点で Remote Execution のための Executor abstraction を導入することはしない。

まずは Local Partition Execution の Execution Semantics と concurrency control を確立する。

---

## 13. Observability

Partition Execution の Execution Semantics と concurrency control が確立した後、OpenTelemetry による観測を追加する。

特に `concurrency` は Step ごとの設定値であるため、Step 単位で現在の並行実行状況を観測できるようにする。

**何を観測するかは、何を実行結果として定義したかによって決まるためである。**

---

## 14. Implementation Roadmap

### Phase 1 — Partition Execution の整理
* 現在の goroutine による並行実行モデルを維持する。
* Partition Generation と Execution の責務を整理する。
* Go の concurrency primitive を利用した構造に整理する。

### Phase 2 — Step-level Concurrency Control
* `PartitionStep` に `concurrency` を導入する。
* 現在の「無制限な並行実行」に対し、semaphore によるリソース制御を追加する。
* Step ごとに最大並行実行数を設定可能にする。
* `context.Context` による cancellation と `errgroup` によるライフサイクル管理を統合する。

### Phase 3 — Partition Execution Semantics
* Partition Worker の状態遷移を明文化
* Controller Step の状態決定を明文化
* Partial Failure / Cancellation / Restart の扱いを定義

### Phase 4 — Partition Failure Matrix
* Partition Execution の状態・失敗パターンを Failure Matrix として整理し、自動テスト化する。

### Phase 5 — OpenTelemetry
* Partition 単位の Observability を追加する。

---

## 15. Design Principles

### 1. Execution Semantics を優先する
Partition の並行実行そのものよりも、「Partition がどのように実行され、失敗したときに何が起きるか」を明確にする。

### 2. Step 単位で concurrency を制御する
Concurrency は Job 全体の設定ではなく、`PartitionStep` ごとの実行パラメータとして扱う。

### 3. Go の concurrency model を利用する
独自 Executor 階層を増やすのではなく、`goroutine`, `context`, `errgroup`, `semaphore` を組み合わせる。

### 4. 実行方式を型で分けない
`SerialPartitionExecutor` や `ParallelPartitionExecutor` のように、実行方式ごとに型を作らない。`concurrency` という一つのパラメータで制御する。

### 5. 抽象化を目的にしない
将来 Remote Execution が必要になる可能性だけを理由に、現時点で Remote Executor の抽象を導入しない。

### 6. Failure Matrix を仕様として扱う
Partition の並行実行は、正常系だけではなく、partial failure, cancellation, restart, checkpoint, idempotency まで含めて定義する。

---

## 16. Summary

Surfin の Partition Execution は、Java/Spring の Executor / Strategy Pattern をそのまま Go に移植するのではなく、

> **JSR-352 の Partitioning が持つ実行意味論を維持しながら、Go の concurrency model で実装する。**

ことを基本方針とする。

Partition の並行実行数は `PartitionStep` ごとに設定する。

**実行意味論は堅牢に、実装は Go らしく軽量に。**

これを Surfin の Partition Execution における基本方針とする。
