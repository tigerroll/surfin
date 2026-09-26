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
*   **ItemListener (Read/Process/Write):** 各アイテムの処理前後のフックを提供。

## 5. Development Roadmap

### 5.1. Target v1.0 (Current)
*   Chunk-oriented processing の安定化。
*   基本的なトランザクション管理とチェックポイント機構の確立。
*   JSR-352 互換のリスナーアーキテクチャの実装。

### 5.2. Target v1.1
*   リモートステップ実行（Remote Partitioning）のサポート強化。
*   OpenTelemetry を活用した分散トレーシングの拡充。
*   動的なフロー制御（Decision/Split）の最適化。

### 5.3. Future Considerations
*   ストリーミング処理への対応。
*   マルチテナント環境におけるジョブ実行の分離。
*   UI/CLI によるジョブ管理・監視ツールの提供。
