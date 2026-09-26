# Surfin Batch Framework: Execution Semantics & Roadmap

本ドキュメントは、Surfin Batch Frameworkにおける実行モデルの仕様（Semantics）と、それを実現するための開発ロードマップを定義します。

## 1. Execution Lifecycle
Surfin Batch Frameworkにおけるステップ実行の標準的なライフサイクルは以下の通りです。

```text
Job Start
  ↓
Step Start (BeforeStep Listener)
  ↓
Open (State Restoration)
  ↓
Chunk Loop (BeforeChunk Listener)
  ↓
Read → Process → Write
  ↓
Transaction Commit
  ↓
ItemStream.Update (Memory Update)
  ↓
saveCheckpoint (Repository Persistence)
  ↓
AfterChunk Listener
  ↓
... (Loop)
  ↓
Close (Final Persistence)
  ↓
Step Complete (AfterStep Listener)
  ↓
Job Complete
```

## 2. State Definitions
各状態は、その時点での「保証」を定義します。

| State | 保証内容 |
| :--- | :--- |
| `STARTING` | ジョブ/ステップが初期化を開始した。 |
| `STARTED` | 実行に必要なリソース（DB接続等）が確保され、処理が開始された。 |
| `COMPLETED` | 全ての処理が正常に終了し、最終チェックポイントが永続化された。 |
| `FAILED` | 処理中に回復不能なエラーが発生した。データはロールバックされている可能性がある。 |
| `STOPPING` | 停止シグナルを受信し、現在のチャンク終了を待機中。 |
| `STOPPED` | 停止シグナルにより安全に停止した。再開可能。 |

## 3. Failure Semantics (Failure Matrix)
障害発生時の挙動を定義します。

| Failure point | Data (DB) | Checkpoint | Step Status |
| :--- | :--- | :--- | :--- |
| **Read** | ロールバックなし | 前回の成功値 | `FAILED` |
| **Process** | ロールバック | 前回の成功値 | `FAILED` |
| **Write** | ロールバック | 前回の成功値 | `FAILED` |
| **Commit** | ロールバック | 前回の成功値 | `FAILED` |
| **Update** | コミット済み | 前回の成功値 | `FAILED` |
| **saveCheckpoint** | コミット済み | 前回の成功値 | `FAILED` |
| **Close** | コミット済み | 前回の成功値/最終値 | `FAILED` |

*注: `Update` 以降の失敗については、データはコミット済みであるため、再開時には「重複処理」が発生する可能性があります。これを防ぐための冪等性はコンポーネント側で担保する必要があります。*

## 4. Design Principles for Implementation

本フレームワークの実装において、以下の設計原則を遵守する。

### 4.1. Update Failure Policy (Policy C)
`ItemStream.Update()` はメモリ上の状態更新のみを責務とし、失敗しない（あるいは失敗を `saveCheckpoint` の失敗として扱う）契約とする。

### 4.2. Failure Matrix Implementation
Failure Matrixの各ケースは、`test/semantics/` 配下に「Executable Specification」として実装済み。
*   **検証状況:** 統合テストにより、各障害ポイントにおける挙動（ロールバック、チェックポイント保存、ステータス遷移）が期待通りであることを保証している。

### 4.3. ExecutionContext Contract
`ExecutionContext` は型安全なアクセサとネスト構造をサポートしており、実行状態の永続化およびコンポーネント間のデータ共有において、この堅牢な契約を遵守すること。

### 4.4. Lifecycle Hooks & Listener Architecture
再実行性および冪等性を安全に担保するため、以下のリスナーアーキテクチャを実装する。
*   **StepExecutionListener:** ステップ全体のセットアップ（BeforeStep）およびクリーンアップ（AfterStep）を担う。
*   **ChunkListener:** トランザクション境界での処理（BeforeChunk, AfterChunk, OnError）を担う。JSR-352仕様に完全準拠。
*   **ItemListener (Read/Process/Write):** 各アイテム処理の前後およびエラー・スキップ時のフックを担う。
*   **重要性:** これらのフックは、ユーザーが冪等性担保ロジック（リソースのクリーンアップ等）を記述するための「安全な場所」を提供する。

## 5. Development Roadmap (EPIC)
Surfinの実行モデルを堅牢化するため、以下のフェーズで開発を進めます。

1.  **Document execution lifecycle** (完了)
2.  **Add normal lifecycle integration tests** (完了)
3.  **Add restart integration tests** (完了)
4.  **Add failure matrix tests** (完了)
5.  **Define Update / saveCheckpoint failure policy** (完了)
6.  **Harden ExecutionContext contract** (完了)
7.  **Implement Lifecycle Listeners (Step/Chunk/Item)** (Completed)
8.  **Define idempotency / duplicate processing semantics** (Next)
9.  **Stateful Processor / Tasklet semantics**
10. **Partition execution semantics**
11. **Batch execution observability**

## 6. Target: Batch Execution Semantics v1 (達成済み)
以下の機能は実装およびテストによる保証が完了しています。

*   Single-threaded Chunk
*   Transaction Management
*   ExecutionContext
*   Checkpointing
*   Restartability
*   Failure Semantics (Failure Matrixの網羅)
*   JSR-352 Compliant Lifecycle Listeners (Step/Chunk/Item)
