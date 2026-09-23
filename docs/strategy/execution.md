# Surfin Batch Framework: Execution Semantics & Roadmap

本ドキュメントは、Surfin Batch Frameworkにおける実行モデルの仕様（Semantics）と、それを実現するための開発ロードマップを定義します。

## 1. Execution Lifecycle
Surfin Batch Frameworkにおけるステップ実行の標準的なライフサイクルは以下の通りです。

```text
Job Start
  ↓
Step Start
  ↓
Open (State Restoration)
  ↓
Chunk Loop (Read → Process → Write)
  ↓
Transaction Commit
  ↓
ItemStream.Update (Memory Update)
  ↓
saveCheckpoint (Repository Persistence)
  ↓
... (Loop)
  ↓
Close (Final Persistence)
  ↓
Step Complete
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
*   **理由:** `Update` をメモリ操作に限定することで、コンポーネント側の実装を単純化し、DB永続化の失敗（`saveCheckpoint`）とロジックの失敗を明確に分離するため。

### 4.2. Failure Matrix Implementation
Failure Matrixの各ケースは、`test/semantics/` 配下に「Executable Specification」として実装する。
*   **実装方針:** `Given/When/Then` 形式の統合テストを作成し、各障害ポイントで「データがロールバックされているか」「チェックポイントが期待通りか」を検証する。
*   **テストの目的:** 異常系における挙動をコードで固定し、将来のリファクタリングによる回帰を防ぐ。

### 4.3. ExecutionContext Contract
`ExecutionContext` は型安全なアクセサとネスト構造をサポートしており、実行状態の永続化およびコンポーネント間のデータ共有において、この堅牢な契約を遵守すること。

## 5. Development Roadmap (EPIC)
Surfinの実行モデルを堅牢化するため、以下のフェーズで開発を進めます。

1.  **Document execution lifecycle** (完了)
2.  **Add normal lifecycle integration tests** (完了)
3.  **Add restart integration tests** (完了)
4.  **Add failure matrix tests** (完了)
5.  **Define Update / saveCheckpoint failure policy** (完了)
6.  **Harden ExecutionContext contract** (完了)
7.  **Define idempotency / duplicate processing semantics**
8.  **Stateful Processor / Tasklet semantics**
9.  **Partition execution semantics**
10. **Batch execution observability**

## 6. Target: Batch Execution Semantics v1
最初のマイルストーンとして、以下の機能を完全にテストで保証することを目指します。

*   Single-threaded Chunk
*   Transaction Management
*   ExecutionContext
*   Checkpointing
*   Restartability
*   Failure Semantics (Failure Matrixの網羅)
