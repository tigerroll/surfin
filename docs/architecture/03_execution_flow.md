# 3. 実行フロー

## 3.1. 起動とJobFactory
1. **DIコンテナ (Fx)**: アプリケーション起動時に、すべてのコンポーネントが初期化され、依存関係が解決されます。
2. **JobFactory**: JSL定義に基づいて実行可能な `core.Job` インスタンスを動的に構築します。

## 3.2. JobLauncherとJobRunner
1. **JobLauncher**: 実行要求を受け取り、`JobInstance` の検索/作成、`JobExecution` の初期化を行います。
2. **JobRunner**: `FlowJob` の実行を担当します。JSLで定義されたフロー（Step, Decision, Split）を辿り、状態を管理します。

## 3.3. StepExecutorの役割
JobRunnerからStepの実行を委譲された `StepExecutor` は、以下の責務を持ちます。
- **SimpleStepExecutor**: ローカルでStepを実行。トランザクション境界を確立。
- **RemoteStepExecutor**: 外部オーケストレーター（例: Kubernetes Job）へ実行を委譲。
