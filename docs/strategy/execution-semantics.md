# Execution Semantics（実行意味論）

## 1. 戦略と基本方針 (Strategy)

* Surfinは単なるバッチ実行ライブラリではなく、バッチ処理における実行動作をフレームワークとして定義する。
* 本ドキュメントは、Surfinがバッチ実行に対してどのような振る舞いを保証するのかを定義し、実装と仕様の乖離を防ぐための戦略を定める。
* 「バッチの実行時に何が起きるかを、各アプリケーションが個別に実装する必要がないようにする」ことを基本方針とする。
* アプリケーションはビジネスロジック（何を読むか、どう処理するか、どこへ書くか）に集中し、エラーハンドリング、トランザクション、リトライ、スキップ、チェックポイント、再開処理などの実行モデルはSurfinが共通基盤として提供する。

## 2. Failure Matrix（障害対応マトリクス）
* Surfinの実行動作の正しさは、以下のFailure Matrixによって定義される。
* このマトリクスは、障害発生時の振る舞いを網羅的に定義した仕様そのものである。

| Operation | Failure Type | Retry | Skip | Transaction State | Expected Result |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Read** | 一時的障害 | Yes | No | N/A | Retry (Backoff) |
| **Read** | 不正データ | No | Yes | N/A | Skip + Log |
| **Process** | 一時的障害 | Yes | No | N/A | Retry (Backoff) |
| **Process** | 不正データ | No | Yes | N/A | Skip + Log |
| **Write** | 一時的障害 | Yes | No | Rollback | Rollback + Retry Chunk |
| **Write** | 不正データ | No | Yes | Rollback | Rollback + Split + Skip Item |
| **Write** | 致命的障害 | No | No | Rollback | Fail |

## 3. 技術仕様リファレンス (Technical Specification)

### 3.1 実行モデル
* Surfinの基本的な実行モデルは以下の通りである。

```text
Job
 └── Step
      │
      ├── Chunk
      │    ├── Read
      │    ├── Process
      │    └── Write
      │
      └── Tasklet
```

* Chunk-oriented Stepのライフサイクルは以下の通りである。

```
BeforeStep → Step STARTED → Checkpoint Restore → Reader/Writer.Open
    │
    ▼
┌──────────────────────┐
│        Chunk         │
│ Read → Process → Write│
└──────────────────────┘
    │
    ▼
Transaction Commit → ItemStream.Update → AfterChunk → Checkpoint Persistence

```

### 3.2 トランザクションと状態管理

* Chunkは基本的なトランザクション境界である。
* 失敗時には、種類に応じてRollback、Retry、Skip、Failが決定される。
* CheckpointはTransaction Commitと密接に関係し、Commitされていない処理を成功済みのCheckpointとして記録してはならない。

### 3.3 障害対応の詳細仕様

* Retry:
  * 再実行で成功する可能性がある処理に適用。
  * Transaction、Idempotencyと密接に関連する。
* Skip:
  * 現在のItemを処理対象から除外し、次のItemへ継続する。
  * Read/Process/Write単位で管理。
* Restart:
  * 失敗したJobExecutionを保存済みの状態から再開する。
  * Transaction、Checkpoint、ExecutionContext、Idempotencyの組み合わせで成立する。
* Idempotency:
  * RetryやRestartによる重複実行を安全に扱うための実行状態管理。

### 3.4 リスナーの振る舞い

* ListenerはSurfinのExecution Lifecycleを構成する要素である。
* 各Listener（Job, Step, Chunk, ItemRead/Process/Write, Skip, Retry）について、成功/失敗/Panic/Retry/Transaction前後/Checkpoint前後での呼び出しを保証する。

## 4. 技術的課題とロードマップ (Semantic Gaps & Roadmap)

### 4.1 未解決の技術的課題 (Semantic Gaps)

* Checkpoint Persistence Failure: 保存失敗時のStep状態（Failureか否か）の定義。
* ItemStream.Update Failure: 失敗時の動作（Step Failure, Warning, Retry等）の定義。
* Listener Failure: Listener自身が失敗した場合のJob/Stepの扱い。
* Panic: ユーザー実装がPanicした場合のTransaction、Status、Checkpointの扱い。
* Retry Backoff: 具体的なBackoff戦略の仕様化。

### 4.2 完了条件

以下の項目を満たした時点を、Execution Semanticsの整備完了とする。

* [x] Job / Step / Chunk / Tasklet Lifecycleの仕様化
* [x] Transaction / Retry / Skip / Checkpoint / Restart / Idempotency / Listenerの仕様化
* [x] Failure Matrixの完成
* [x] Failure Matrixの各ケースを自動テスト化 (test/semantics/failure_matrix_test.go および test/pkg/batch/core/fault_tolerance_test.go にて網羅)
* [x] CIで継続的に検証
* [x] Execution Semantics変更時に仕様とテストの更新を必須化

**ステータス: 全項目完了済み**
