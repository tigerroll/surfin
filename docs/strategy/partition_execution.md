# Strategy Document: Partition Execution Strategy

## 1. 目的
現在の `PartitionStep` 実装は既に並列実行を実現していますが、大規模バッチ処理におけるリソース制御や、将来的なリモート実行への対応を見据え、実行ロジックを抽象化・戦略化することを目的とします。本戦略により、テスト容易性の向上と、環境に応じた柔軟な実行制御を実現します。

## 2. 設計原則
*   **意味論の不変性 (Semantics Preservation)**: リファクタリング前後でジョブの実行結果や障害時の振る舞い（Failure Matrix）を一切変更しない。
*   **安全性優先 (Safety First / Serial by Default)**: 外部APIのレートリミットやDB負荷を考慮し、デフォルトでは直列実行（Serial）を推奨する。並列実行はスループット向上のための「オプション」として位置づける。
*   **責務の分離**: `PartitionStep` は「パーティションの生成と結果集約」に集中し、「実行戦略（直列/並列/リモート）」は `PartitionExecutor` に委譲する。
*   **論理と物理の分離**: 「パーティション数（論理的な分割数）」と「同時実行数（物理的なリソース制御）」を分離し、環境に応じたチューニングを可能にする。

## 3. 現状の課題
*   **リソース制御の欠如:** 現在の並列実行は `go func()` を無制限に起動するため、大規模なジョブにおいてリソース枯渇（DB接続数超過やメモリ不足）のリスクがあります。
*   **責務の混在:** `PartitionStep` が「パーティション生成」「実行制御」「結果集約」のすべてを担っており、コードが肥大化しています。
*   **実行戦略の固定:** 並列実行がハードコードされており、外部APIのレートリミット（429エラー）やDBロック競合が発生した際に、即座にシリアル実行へ切り替えることが困難です。

## 4. 提案するアーキテクチャ
Strategy パターンを導入し、実行ロジックを `PartitionExecutor` インターフェースとして分離します。

### 4.1. 抽象化レイヤー
```go
type PartitionExecutor interface {
    Execute(
        ctx context.Context,
        partitions map[string]model.ExecutionContext,
        workerStep port.Step,
        jobExecution *model.JobExecution,
        controllerExecution *model.StepExecution,
    ) (<-chan *model.StepExecution, <-chan error)
}
```

### 4.2. 実装戦略のバリエーション
*   **`SerialPartitionExecutor`**: **【推奨デフォルト】** 本番環境での安定稼働および外部APIのレートリミット回避のための戦略。パーティションを順次実行し、決定論的な挙動を保証します。
*   **`ParallelPartitionExecutor`**: スループット向上が必要な場合のオプション戦略。セマフォ（`golang.org/x/sync/semaphore`）を用いて同時実行数を制御し、リソース枯渇を防ぎます。
*   **`RemotePartitionExecutor`**: 将来的な拡張用。リモートエンジンへジョブを委譲します。

## 5. 実装ロードマップ

### Phase 1: リファクタリング（意味論の維持）
*   **目的**: 既存の `PartitionStep` 内の並列実行ロジックを `PartitionExecutor` へ移行する。
*   **内容**:
    *   `PartitionExecutor` インターフェースの定義。
    *   既存の `go func()` による並列実行ロジックを `ParallelPartitionExecutor` へ移動。
    *   `PartitionStep` が `ParallelPartitionExecutor` をデフォルトで使用するように注入。
*   **制約**: 挙動を一切変えないこと。Failure Matrix に影響を与えないことを最優先とする。

### Phase 2: 実行制御の強化と戦略の拡充
*   **目的**: リソース制約環境への対応と、実行戦略の切り替えを実現する。
*   **内容**:
    *   **セマフォの導入**: `ParallelPartitionExecutor` に同時実行数制限を実装。
    *   **Serial 戦略の実装**: `SerialPartitionExecutor` を実装し、1つずつ逐次実行する戦略を追加。
    *   **Fx 統合**: Fx のプロバイダー設定により、環境変数や設定ファイルに基づいて `Serial` / `Parallel` を動的に切り替え可能にする。

### Phase 3: リモート実行への拡張
*   **目的**: 分散環境での実行サポート。
*   **内容**:
    *   `RemotePartitionExecutor` の実装。
    *   既存の `RemoteJobSubmitter` との統合。

## 6. 期待される効果
*   **運用上の安全性:** 外部APIのレートリミット（429エラー）やDBのロック競合を、コード変更なしで設定変更（Serialへの切り替え）のみで回避可能。
*   **テスト容易性:** `SerialPartitionExecutor` を使用することで、並列処理特有の競合を排除した単体テストが可能になります。
*   **安定性:** セマフォによる同時実行数制限により、システム全体の負荷を予測可能にします。
*   **拡張性:** 新しい実行エンジン（リモート実行等）を追加する際、`PartitionStep` 本体を修正することなく、新しい Executor を実装するだけで対応可能になります。
