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

### 3.5 Retry Backoff Strategy (リトライバックオフ戦略)

リトライ発生時、システム負荷の軽減と成功率の向上を目的として、バックオフ（待機時間）を導入する。Surfin におけるバックオフは、単なる待機ではなく「実行意味論の一部」として以下の仕様を遵守する。

#### 3.5.1 基本仕様

| 項目 | 仕様 |
| :--- | :--- |
| **タイミング** | Retry attempt の**前**に待機する。 |
| **初回リトライ** | `attempt=1` のバックオフ時間を適用する。 |
| **Context 尊重** | 待機中であっても `context.Context` のキャンセルを監視し、即座に中断する。 |
| **Skipとの関係** | Skip される場合はバックオフを行わない。 |

#### 3.5.2 設計パターン: BackoffWaiter

テスト容易性を担保するため、実際の待機処理は `BackoffWaiter` インターフェースを介して注入する。これにより、プロダクション環境では実時間待機を行い、テスト環境では即時完了するモックを利用可能とする。

```go
// BackoffWaiter は、リトライ間の待機処理を抽象化するインターフェースです。
type BackoffWaiter interface {
    // Wait は、指定された期間待機します。Context がキャンセルされた場合は即座にエラーを返します。
    Wait(ctx context.Context, duration time.Duration) error
}
```

#### 3.5.3 実装上の注意
* `time.Sleep()` を直接使用してはならない。必ず `BackoffWaiter` を経由し、`context.Context` を尊重すること。
* バックオフ計算ロジックと待機処理（副作用）を分離し、テスト時には待機時間をスキップできるように設計すること。

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
