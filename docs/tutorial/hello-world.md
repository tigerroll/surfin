# 👉 tutorial - "Hello, World!"

#### 概要

このチュートリアルでは、Surfin Batch Framework を使用して、コンソールに「Hello, World!」と出力するだけの最もシンプルなバッチアプリケーションを構築します。特に、データベース接続を一切行わない「DBレス」モードでの実行に焦点を当て、フレームワークの基本的なコンポーネントの連携と、DB依存を排除するための設定方法を学びます。

#### 学習目標

*   Surfin Batch の基本的なプロジェクト構造を理解する。
*   JSL (Job Specification Language) を用いたジョブ定義の基本を学ぶ。
*   カスタム Tasklet コンポーネントを実装し、フレームワークに登録する方法を学ぶ。
*   データベースを使用しない環境で Surfin Batch を実行するための設定方法を理解する。

#### 前提条件

*   Go 言語の基本的な知識
*   Go Modules の使用経験
*   `Task` (Taskfile) コマンドラインツールのインストール
*   Surfin Batch Framework のリポジトリをクローン済みであること

---

### 1. プロジェクトのセットアップ

このチュートリアルでは、Surfin Batch Framework のルートディレクトリ直下に新しいプロジェクトディレクトリ `example/hello-world` を作成し、必要なファイルをゼロから構築します。

```bash
# Surfin Batch Framework のルートディレクトリにいることを確認してください
# 例: ~/go/src/surfin

# 新しいプロジェクトディレクトリと必要なサブディレクトリを作成
mkdir -p example/hello-world/cmd/hello-world/resources
mkdir -p example/hello-world/internal/app
mkdir -p example/hello-world/internal/step

# 新しいプロジェクトディレクトリに移動
cd example/hello-world

# Goモジュールを初期化
# モジュール名は Surfin Batch Framework のルートモジュール名に続けて、新しいプロジェクトのパスを指定します。
go mod init surfin/example/hello-world

# 必要なGoモジュールの依存関係を追加
go get go.uber.org/fx
go get gopkg.in/yaml.v3
go get github.com/google/uuid
go mod tidy
```

次に、以下のファイルをそれぞれのパスに**新規作成**し、内容を記述してください。

#### `example/hello-world/cmd/hello-world/resources/application.yaml`

```yaml
surfin:
  system:
    timezone: Asia/Tokyo
    logging:
      level: DEBUG
  batch:
    job_name: helloWorldJob
    polling_interval_seconds: 5
    chunk_size: 1000
    item_retry:
      max_attempts: 3
      initial_interval: 100
      retryable_exceptions:
        - "BatchError"
    item_skip:
      skip_limit: 10
      skippable_exceptions:
        - "BatchError" # In-memory DB のため、datasources および infrastructure セクションは不要
  security:
    masked_parameter_keys:
      - "password"
      - "api_key"
```

#### `example/hello-world/Taskfile.yaml`

```yaml
version: '3'

vars:
  APP_MODULE_PATH: surfin/example/hello-world/cmd/hello-world
  APP_BINARY_NAME: hello-world
  BUILD_OUTPUT_DIR: ../../dist

tasks:
  default:
    desc: "List tasks for the hello-world application."
    cmds:
      - task --list

  build:
    desc: "Build the hello-world application executable."
    cmds:
      - go build -v -gcflags="all=-N -l" -o {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}} {{.APP_MODULE_PATH}}
      - echo "Built {{.APP_BINARY_NAME}} to {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"
    generates: ["{{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"]
    sources:
      - "./cmd/hello-world/**/*.go"
      - "./internal/**/*.go"
      - "./cmd/hello-world/resources/application.yaml"
      - "./cmd/hello-world/resources/job.yaml"

  run:
    desc: "Run the hello-world application."
    deps: [build]
    cmds: ["{{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"]
    env:
      BATCH_LOG_LEVEL: DEBUG

  clean:
    desc: "Remove build artifacts for hello-world application."
    cmds:
      - rm -f {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}

  test:
    desc: "Run tests for the hello-world application."
    cmds:
      - go test ./internal/... -v -count=1
```

以降の変更は、この `example/hello-world` ディレクトリ内で行うことを想定しています。

### 2. JSL (Job Specification Language) の定義

ジョブの実行フローを定義する `job.yaml` を修正し、シンプルな「Hello, World!」Taskletを実行するように設定します。

`example/hello-world/cmd/hello-world/resources/job.yaml` を以下の内容で**新規作成**してください。

```yaml # example/hello-world/cmd/hello-world/resources/job.yaml
id: helloWorldJob
name: Hello World Batch Job
description: This job simply prints "Hello, World!" to the console.

flow:
  start-element: helloWorldStep
  elements:
    helloWorldStep:
      id: helloWorldStep
      tasklet:
        ref: helloWorldTasklet
        properties:
          message: "Hello, Surfin Batch World!"
      listeners:
        - ref: loggingStepListener
      transitions:
        - on: COMPLETED
          end: true
        - on: FAILED
          fail: true
```

**内容のポイント:**
*   `id`と`name`を`helloWorldJob`に設定しました。
*   `flow`セクションを簡素化し、`start-element`を`helloWorldStep`に設定しました。
*   `elements`に`helloWorldStep`を定義し、`tasklet`の`ref`を`helloWorldTasklet`に指定しました。
*   `properties`に`message`キーを追加し、Taskletに渡す文字列を定義しました。
*   `listeners`として`loggingStepListener`を追加し、ステップのライフサイクルイベントをログに出力するようにしました。
*   `transitions`で、ステップが`COMPLETED`したらジョブを終了し、`FAILED`したらジョブを失敗として終了するように設定しました。

### 3. カスタム Tasklet の実装

「Hello, World!」メッセージをログに出力する Tasklet を実装します。

`example/hello-world/internal/step/hello_tasklet.go` を以下の内容で**新規作成**してください。

```go # example/hello-world/internal/step/hello_tasklet.go
package step

import (
	"context"
	"fmt"

	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
)

// HelloWorldTaskletConfig は JSL から渡されるプロパティをバインドするための構造体です。
type HelloWorldTaskletConfig struct {
	Message string `yaml:"message"` // JSLのproperties.messageに対応
}

// HelloWorldTasklet はシンプルなTaskletの実装です。
type HelloWorldTasklet struct {
	config *HelloWorldTaskletConfig
	executionContext model.ExecutionContext // Taskletの内部状態を保持するためのExecutionContext
}

// NewHelloWorldTasklet は HelloWorldTasklet の新しいインスタンスを作成します。
func NewHelloWorldTasklet(properties map[string]string) (*HelloWorldTasklet, error) {
	taskletCfg := &HelloWorldTaskletConfig{}
	
	if err := configbinder.BindProperties(properties, taskletCfg); err != nil { // JSLプロパティを構造体にバインドします。
		// isSkippable と isRetryable は false に設定
		return nil, exception.NewBatchError("hello_world_tasklet", "Failed to bind properties", err, false, false)
	}

	if taskletCfg.Message == "" {
		return nil, fmt.Errorf("message property is required for HelloWorldTasklet")
	}

	return &HelloWorldTasklet{
		config: taskletCfg,
		executionContext: model.NewExecutionContext(),
	}, nil
}

// Execute は Tasklet の主要なロジックを実行します。
func (t *HelloWorldTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return model.ExitStatusFailed, ctx.Err()
	default:
	}

	logger.Infof("HelloWorldTasklet: %s", t.config.Message)
	return model.ExitStatusCompleted, nil
}

// Close はリソースを解放します。
func (t *HelloWorldTasklet) Close(ctx context.Context) error {
	logger.Debugf("HelloWorldTasklet: Close called.")
	return nil
}

// SetExecutionContext は Tasklet の ExecutionContext を設定します。
func (t *HelloWorldTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.executionContext = ec
	return nil
}

// GetExecutionContext は Tasklet の ExecutionContext を取得します。
func (t *HelloWorldTasklet) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return t.executionContext, nil
}
```

**内容のポイント:**
*   `HelloWorldTasklet`構造体は、JSLから`message`プロパティを受け取る`HelloWorldTaskletConfig`を含んでいます。
*   `NewHelloWorldTasklet`コンストラクタは、`configbinder.BindProperties`を使用してJSLのプロパティを`HelloWorldTaskletConfig`にバインドします。
*   `Execute`メソッド内で、JSLから渡されたメッセージを`logger.Infof`で出力します。
*   `SetExecutionContext`と`GetExecutionContext`は、このTaskletでは状態管理が不要なため、最小限の実装となっています。

### 4. カスタム Tasklet のフレームワークへの登録

作成した `HelloWorldTasklet` を Surfin Batch Framework の DI コンテナ (Fx) に登録するためのモジュールを作成します。

`example/hello-world/internal/step/hello_tasklet_module.go` を以下の内容で**新規作成**してください。

```go # example/hello-world/internal/step/hello_tasklet_module.go
package step

import (
	"go.uber.org/fx"

	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	job "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewHelloWorldTaskletComponentBuilder は HelloWorldTasklet の jsl.ComponentBuilder を作成します。
func NewHelloWorldTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		repo job.JobRepository,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		_ = cfg // このコンポーネントでは不要な引数は無視します。
		_ = repo
		_ = resolver
		_ = dbResolver

		tasklet, err := NewHelloWorldTasklet(properties)
		if err != nil {
			return nil, err
		}
		return tasklet, nil
	})
}

// RegisterHelloWorldTaskletBuilder は作成した ComponentBuilder を JobFactory に登録します。
func RegisterHelloWorldTaskletBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) {
	jf.RegisterComponentBuilder("helloWorldTasklet", builder) // JSL (job.yaml) の 'ref: helloWorldTasklet' と一致するキーでビルダを登録します。
}

// Module は HelloWorldTasklet コンポーネントの Fx オプションを定義します。
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewHelloWorldTaskletComponentBuilder,
		fx.ResultTags(`name:"helloWorldTasklet"`),
	)),
	fx.Invoke(fx.Annotate(
		RegisterHelloWorldTaskletBuilder,
		fx.ParamTags(``, `name:"helloWorldTasklet"`),
	)),
)
```

**内容のポイント:**
*   `NewHelloWorldTaskletComponentBuilder`関数は、`jsl.ComponentBuilder`インターフェースを実装する関数を返します。この関数は、`JobFactory`がTaskletのインスタンスを作成する際に呼び出されます。
*   `RegisterHelloWorldTaskletBuilder`関数は、`JobFactory`に`helloWorldTasklet`という名前でビルダを登録します。この名前は`job.yaml`で指定した`ref`と一致する必要があります。
*   `Module`変数は、FxアプリケーションにこのTaskletを組み込むための`fx.Options`を提供します。

### 4.5. アプリケーション固有のジョブとランナーの定義

Surfin Batch Framework では、ジョブの実行ロジックを定義する `port.Job` インターフェースの実装と、そのジョブのフローを実際に実行する `port.JobRunner` の実装が必要です。このチュートリアルでは、`HelloWorldJob` と `FlowJobRunner` を使用します。

#### `example/hello-world/internal/app/job/hello_world_job.go`

このファイルは、`port.Job` インターフェースを実装し、`helloWorldJob` の基本的な情報（ID、名前、フロー定義など）を保持します。実際のジョブ実行ロジックは `JobRunner` に委譲されるため、`Run` メソッドはシンプルです。

```go # example/hello-world/internal/app/job/hello_world_job.go
package job

import (
	"context"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HelloWorldJob は port.Job インターフェースを実装するシンプルなジョブです。
// このチュートリアル用に JobRunner のロジックを直接含まず、
// JobFactory から渡される FlowDefinition を保持します。
type HelloWorldJob struct {
	id             string
	name           string
	flow           *model.FlowDefinition
	jobRepository  repository.JobRepository
	cfg            *config.Config
	listeners      []port.JobExecutionListener
	metricRecorder metrics.MetricRecorder
	tracer         metrics.Tracer
}

// NewHelloWorldJob は HelloWorldJob の新しいインスタンスを作成します。
func NewHelloWorldJob(
	jobRepository repository.JobRepository,
	cfg *config.Config,
	listeners []port.JobExecutionListener,
	flow *model.FlowDefinition,
	metricRecorder metrics.MetricRecorder,
	tracer metrics.Tracer,
) (port.Job, error) {
	return &HelloWorldJob{
		id:             "helloWorldJob", // JSLのIDと一致
		name:           "Hello World Batch Job",
		flow:           flow,
		jobRepository:  jobRepository,
		cfg:            cfg,
		listeners:      listeners,
		metricRecorder: metricRecorder,
		tracer:         tracer,
	}, nil
}

// Run はジョブの実行ロジックです。
// SimpleJobLauncher が JobRunner を使ってこの Run を呼び出すため、
// ここでは直接フローを実行するのではなく、JobRunner に処理を委譲します。
// ただし、この Job の実装は JobRunner を直接参照しないため、
// ここでは何もしないか、ログを出す程度に留めます。
func (j *HelloWorldJob) Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error {
	logger.Infof("HelloWorldJob.Run called for JobExecution ID: %s", jobExecution.ID)
	// 実際のフロー実行は JobRunner が行います。
	return nil
}

// JobName はジョブの論理名を返します。
func (j *HelloWorldJob) JobName() string {
	return j.name
}

// ID はジョブ定義のユニークなIDを返します。
func (j *HelloWorldJob) ID() string {
	return j.id
}

// GetFlow はジョブのフロー定義構造を返します。
func (j *HelloWorldJob) GetFlow() *model.FlowDefinition {
	return j.flow
}

// ValidateParameters はジョブ実行前にジョブパラメータを検証します。
func (j *HelloWorldJob) ValidateParameters(params model.JobParameters) error {
	// このチュートリアルではパラメータ検証は不要
	return nil
}

// HelloWorldJob が port.Job インターフェースを実装していることを確認します。
var _ port.Job = (*HelloWorldJob)(nil)
```

#### `example/hello-world/internal/app/job/module.go`

このファイルは、`HelloWorldJob` を Fx コンテナに登録し、`JobFactory` がこのジョブをインスタンス化できるようにします。

```go # example/hello-world/internal/app/job/module.go
package job

import "go.uber.org/fx"
import support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
import logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

// RegisterHelloWorldJobBuilder は作成した JobBuilder を JobFactory に登録します。
func RegisterHelloWorldJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	jf.RegisterJobBuilder("helloWorldJob", builder) // JobFactory に JobBuilder を登録
	logger.Debugf("JobBuilder for helloWorldJob registered with JobFactory. JSL id: 'helloWorldJob'") // JSL (job.yaml) の 'id: helloWorldJob' と一致するキーでビルダを登録します。
}

// provideHelloWorldJobBuilder は NewHelloWorldJob 関数を support.JobBuilder 型として提供します。
// NewHelloWorldJob の依存関係は、この関数が返す JobBuilder が実際に呼び出される際に解決されます。
func provideHelloWorldJobBuilder() support.JobBuilder {
	return NewHelloWorldJob
}

// Module は helloWorldJob コンポーネントの Fx オプションを定義します。
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		provideHelloWorldJobBuilder, // provideHelloWorldJobBuilder 関数が support.JobBuilder 型を返します
		fx.ResultTags(`name:"helloWorldJob"`), // JobFactory がこの名前で JobBuilder を取得できるようにタグ付け
	)),
	fx.Invoke(fx.Annotate(
		RegisterHelloWorldJobBuilder,
		fx.ParamTags(``, `name:"helloWorldJob"`),
	)),
)
```

#### `example/hello-world/internal/app/runner/flow_job_runner.go`

`FlowJobRunner` は、ジョブのフロー定義（ステップ、決定、分割など）に基づいてジョブを実行する主要なコンポーネントです。

```go # example/hello-world/internal/app/runner/flow_job_runner.go
package runner

import (
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	exception "github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// FlowJobRunner is an implementation of JobRunner that executes a job based on its flow definition.
type FlowJobRunner struct {
	jobRepository repository.JobRepository
	stepExecutor  port.StepExecutor
	tracer        metrics.Tracer
}

// NewFlowJobRunner creates a new FlowJobRunner.
func NewFlowJobRunner(
	repo repository.JobRepository,
	executor port.StepExecutor,
	tracer metrics.Tracer,
) *FlowJobRunner {
	return &FlowJobRunner{
		jobRepository: repo,
		stepExecutor:  executor,
		tracer:        tracer,
	}
}

// Run starts the execution according to the job's flow definition.
// This method orchestrates the job flow by executing steps, decisions, and splits.
func (r *FlowJobRunner) Run(ctx context.Context, jobInstance port.Job, jobExecution *model.JobExecution, flowDef *model.FlowDefinition) {
	logger.Infof("FlowJobRunner: Starting execution for Job '%s' (Execution ID: %s).", jobInstance.JobName(), jobExecution.ID)

	// JobExecution のステータスを STARTED に更新
	jobExecution.MarkAsStarted()
	if err := r.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
		logger.Errorf("FlowJobRunner: Failed to update JobExecution status to STARTED: %v", err)
		jobExecution.MarkAsFailed(err)
		r.jobRepository.UpdateJobExecution(ctx, jobExecution) // 最終ステータスを保存を試みる
		return
	}

	// ジョブ実行のトレーシングスパンを開始
	jobCtx, endJobSpan := r.tracer.StartJobSpan(ctx, jobExecution)
	defer endJobSpan()

	// 開始要素を取得
	currentElementID := flowDef.StartElement
	var currentElement interface{}
	var ok bool

	for {
		select {
		case <-jobCtx.Done():
			logger.Warnf("FlowJobRunner: Job context cancelled for Job '%s' (Execution ID: %s).", jobInstance.JobName(), jobExecution.ID)
			jobExecution.MarkAsStopped()
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		default:
			// 続行
		}

		currentElement, ok = flowDef.Elements[currentElementID]
		if !ok {
			err := exception.NewBatchErrorf("flow_runner", "Flow element '%s' not found in flow definition for job '%s'", currentElementID, jobInstance.JobName())
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		}

		var exitStatus model.ExitStatus
		var elementErr error

		switch element := currentElement.(type) {
		case port.Step:
			logger.Infof("FlowJobRunner: Executing Step '%s' for Job '%s'.", element.StepName(), jobInstance.JobName())

			// 新しい StepExecution を作成
			stepExecution := model.NewStepExecution(model.NewID(), jobExecution, element.StepName())
			jobExecution.AddStepExecution(stepExecution) // JobExecution のリストに追加
			jobExecution.CurrentStepName = element.StepName() // 現在のステップ名を更新

			// StepExecution を最初に保存する (SimpleStepExecutor が保存しない場合のワークアラウンド)
			// StepExecutor がトランザクション内でこれを処理すべきだが、現在の実装では不足しているため、ここで補完する。
			// これにより、TaskletStep/ChunkStep 内での最初の UpdateStepExecution 呼び出しが成功するようになる。
			if err := r.jobRepository.SaveStepExecution(jobCtx, stepExecution); err != nil {
				elementErr = exception.NewBatchError(element.StepName(), "Failed to save initial StepExecution", err, false, false)
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Failed to save initial StepExecution for Step '%s': %v", element.StepName(), err)
				jobExecution.MarkAsFailed(elementErr)
				r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
				return // Run メソッドを終了
			}
			// ステップを実行
			executedStepExecution, err := r.stepExecutor.ExecuteStep(jobCtx, element, jobExecution, stepExecution)
			if err != nil {
				elementErr = err
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Step '%s' failed: %v", element.StepName(), err)
			} else {
				exitStatus = executedStepExecution.ExitStatus
				logger.Infof("FlowJobRunner: Step '%s' completed with ExitStatus: %s", element.StepName(), exitStatus)
			}

			// Step から Job への ExecutionContext の昇格
			if promotion := element.GetExecutionContextPromotion(); promotion != nil {
				for _, key := range promotion.Keys {
					if val, ok := executedStepExecution.ExecutionContext.Get(key); ok {
						jobExecution.ExecutionContext.Put(key, val)
					}
				}
				for stepKey, jobKey := range promotion.JobLevelKeys {
					if val, ok := executedStepExecution.ExecutionContext.Get(stepKey); ok {
						jobExecution.ExecutionContext.Put(jobKey, val)
					}
				}
			}

		case port.Decision:
			logger.Infof("FlowJobRunner: Executing Decision '%s' for Job '%s'.", element.DecisionName(), jobInstance.JobName())
			// 次のパスを決定
			decisionExitStatus, err := element.Decide(jobCtx, jobExecution, jobExecution.Parameters)
			if err != nil {
				elementErr = err
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Decision '%s' failed: %v", element.DecisionName(), err)
			} else {
				exitStatus = decisionExitStatus
				logger.Infof("FlowJobRunner: Decision '%s' resulted in ExitStatus: %s", element.DecisionName(), exitStatus)
			}

		case port.Split:
			logger.Infof("FlowJobRunner: Executing Split '%s' for Job '%s'.", element.ID(), jobInstance.JobName())
			// TODO: Split の並列実行を実装
			// 現時点では、未実装としてエラーを返す
			elementErr = exception.NewBatchErrorf("flow_runner", "Split execution is not yet implemented for Split '%s'", element.ID())
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)

		default:
			elementErr = exception.NewBatchErrorf("flow_runner", "Unknown flow element type for ID '%s': %T", currentElementID, currentElement)
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)
		}

		// 次の遷移ルールを検索
		nextRule, found := flowDef.GetTransitionRule(currentElementID, exitStatus, elementErr != nil)
		if !found {
			// 特定のルールが見つからない場合、ワイルドカードまたはデフォルトを試す
			nextRule, found = flowDef.GetTransitionRule(currentElementID, model.ExitStatusUnknown, elementErr != nil) // '*' を確認
		}

		if !found {
			// 遷移ルールが見つからない場合、ジョブは失敗として終了
			err := exception.NewBatchErrorf("flow_runner", "No transition rule found for element '%s' with ExitStatus '%s' (error: %v)", currentElementID, exitStatus, elementErr)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		}

		// 遷移ルールを適用
		if nextRule.Transition.End {
			jobExecution.MarkAsCompleted()
			if elementErr != nil { // エラーがあったが 'end' 遷移の場合、それでも完了とする
				jobExecution.AddFailureException(elementErr)
			}
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) completed with ExitStatus: %s (Transition: END).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // ループを終了
		} else if nextRule.Transition.Fail {
			jobExecution.MarkAsFailed(elementErr)
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) failed with ExitStatus: %s (Transition: FAIL).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // ループを終了
		} else if nextRule.Transition.Stop {
			jobExecution.MarkAsStopped()
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) stopped with ExitStatus: %s (Transition: STOP).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // ループを終了
		} else if nextRule.Transition.To != "" {
			currentElementID = nextRule.Transition.To
			logger.Debugf("FlowJobRunner: Transitioning to next element: '%s'", currentElementID)
		} else {
			// バリデーションが正しければ発生しないはずだが、念のため
			err := exception.NewBatchErrorf("flow_runner", "Invalid transition rule for element '%s': no 'to', 'end', 'fail', or 'stop' specified", currentElementID)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			break // ループを終了
		}
	}

	// JobExecution の最終更新 (ブレーク条件で既に更新されていない場合)
	if !jobExecution.Status.IsFinished() {
		jobExecution.MarkAsCompleted() // 明示的な終了ステータスなしでループが終了した場合、完了と見なす
	}
	if err := r.jobRepository.UpdateJobExecution(jobCtx, jobExecution); err != nil {
		logger.Errorf("FlowJobRunner: Failed to update final JobExecution status: %v", err)
	}
	logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) finished with status: %s, ExitStatus: %s",
		jobInstance.JobName(), jobExecution.ID, jobExecution.Status, jobExecution.ExitStatus)
}
```

#### `example/hello-world/internal/app/runner/module.go`

このファイルは、`FlowJobRunner` を Fx コンテナに登録し、`port.JobRunner` インターフェースの実装として提供します。

```go # example/hello-world/internal/app/runner/module.go
package runner

import (
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	"go.uber.org/fx"
)

// FlowJobRunnerParams defines dependencies for FlowJobRunner.
type FlowJobRunnerParams struct {
	fx.In
	JobRepository repository.JobRepository
	StepExecutor  port.StepExecutor
	Tracer        metrics.Tracer
}

// NewJobRunner provides the concrete JobRunner implementation (FlowJobRunner).
func NewJobRunner(p FlowJobRunnerParams) port.JobRunner {
	return NewFlowJobRunner(p.JobRepository, p.StepExecutor, p.Tracer)
}

// Module provides the JobRunner implementation.
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		NewJobRunner,
		fx.As(new(port.JobRunner)),
	)),
)
```

### 5. データベース依存の排除とダミー実装

Surfin Batch は `JobRepository` を介してジョブのメタデータを管理しますが、DBレスモードではこれをフレームワークが提供する**インメモリ実装**に置き換えます。これにより、永続的なデータベース接続が不要になります。また、このチュートリアルでは明示的なデータベース接続設定を行わないため、フレームワークは自動的にデータベース関連のインターフェースに対して**ダミー実装**を使用します。

#### 5.1. インメモリ JobRepository の利用

このチュートリアルでは、データベースを使用しない「DBレス」モードでバッチを実行します。Surfin Batch は `JobRepository` を介してジョブのメタデータを管理しますが、DBレスモードではこれをフレームワークが提供する**インメモリ実装**に置き換えます。

以前のチュートリアルでは、このインメモリ実装を独自に作成していましたが、Surfin Batch フレームワークにはすでに `pkg/batch/infrastructure/repository/inmemory` パッケージにインメモリの `JobRepository` 実装が用意されています。これを利用することで、より簡潔にDBレスモードを設定できます。

#### 5.2. アプリケーションの Fx 設定 (`app_options.go`)
アプリケーションの Fx コンテナを構築するためのオプションは、`main` パッケージ内の `app_options.go` にある `GetApplicationOptions` 関数で定義されます。この関数は、アプリケーションに必要なすべての Fx オプションを返します。

`example/hello-world/cmd/hello-world/app_options.go` を以下の内容で**新規作成**してください。

```go # example/hello-world/cmd/hello-world/app_options.go
package main

import (
	"context"

	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	decision "github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	split "github.com/tigerroll/surfin/pkg/batch/core/job/split"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	incrementer "github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	jobRunner "github.com/tigerroll/surfin/pkg/batch/core/job/runner"
	inmemoryRepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/inmemory"
	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	dummy "github.com/tigerroll/surfin/pkg/batch/adaptor/database/dummy"
	
	"go.uber.org/fx"

	appjob "github.com/tigerroll/surfin/example/hello-world/internal/app/job"
)

// GetApplicationOptions は uber-fx のオプションを構築し、スライスとして返します。
// この関数は fx.New の呼び出しの前に定義されている必要があります。
func GetApplicationOptions(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes) []fx.Option {
	cfg, err := config.LoadConfig(envFilePath, embeddedConfig)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}
	logger.SetLogLevel(cfg.Surfin.System.Logging.Level)
	logger.Infof("Log level set to: %s", cfg.Surfin.System.Logging.Level)

	var options []fx.Option

	options = append(options, fx.Supply(
		embeddedConfig,
		embeddedJSL,
		fx.Annotate(envFilePath, fx.ResultTags(`name:"envFilePath"`)),
		cfg,
		fx.Annotate(appCtx, fx.As(new(context.Context)), fx.ResultTags(`name:"appCtx"`)),
	))
	options = append(options, logger.Module)
	options = append(options, config.Module)
	options = append(options, metrics.Module)
	options = append(options, bootstrap.Module)
	options = append(options, fx.Provide(supportConfig.NewJobFactory))
	options = append(options, usecase.Module)
	options = append(options, inmemoryRepo.Module)
	options = append(options, batchlistener.Module)
	options = append(options, decision.Module)
	options = append(options, split.Module)
	options = append(options, apprunner.Module)
	options = append(options, incrementer.Module)
	options = append(options, item.Module)
	options = append(options, fx.Invoke(fx.Annotate(startJobExecution, fx.ParamTags("", "", "", "", "", `name:"appCtx"`))))
	options = append(options, helloTasklet.Module)
	options = append(options, appjob.Module) // アプリケーション固有の JobBuilder を提供するモジュールを直接追加
	options = append(options, apprunner.Module) // apprunner.Module を追加
	return options
}
```

**内容のポイント:**
*   `GetApplicationOptions`関数は、アプリケーションのFxコンテナを構築するためのオプションを `fx.Option` 型として返します。
*   `jobRepo.JobRepository`の実装として、`inmemoryRepo.Module`（フレームワーク提供のインメモリリポジトリ）が使用されます。これにより、データベースへの永続化が不要になります。
*   `dummy.Module`が追加され、DB関連のインターフェースに対するダミー実装が提供されます。
*   `helloTasklet.Module`がFxオプションに追加され、カスタムTaskletがアプリケーションに組み込まれます。

#### 5.3. main関数の内容 (`main.go`)
`main.go` は、`GetApplicationOptions` 関数から取得したオプションを使用して Fx アプリケーションを初期化し、実行します。これにより、アプリケーションの起動ロジックが明確に定義されます。

`example/hello-world/cmd/hello-world/main.go` を以下の内容で**更新**してください。

```go # example/hello-world/cmd/hello-world/main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "embed"

	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	
	"go.uber.org/fx"
)

// embeddedConfig はアプリケーションのYAML設定ファイルの内容を埋め込みます。
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// embeddedJSL はジョブ仕様言語 (JSL) ファイルの内容を埋め込みます。
//
//go:embed resources/job.yaml
var embeddedJSL []byte


// startJobExecution はアプリケーション起動時にジョブ実行を開始する Fx Hook ヘルパー関数です。
func startJobExecution(
    lc fx.Lifecycle,
    shutdowner fx.Shutdowner,
    jobLauncher *usecase.SimpleJobLauncher, // Concrete type used
    jobRepository jobRepo.JobRepository,
    cfg *config.Config,
    appCtx context.Context,
) {
	lc.Append(fx.Hook{
		OnStart: onStartJobExecution(jobLauncher, jobRepository, cfg, shutdowner, appCtx),
		OnStop:  onStopApplication(),
	})
}

// onStartJobExecution is an Fx Hook helper function that starts job execution upon application startup.
func onStartJobExecution(
    jobLauncher *usecase.SimpleJobLauncher, // Concrete type used
    jobRepository jobRepo.JobRepository,
    cfg *config.Config,
    shutdowner fx.Shutdowner,
    appCtx context.Context,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Panic recovered in job execution: %v", r)
				}
				logger.Infof("Requesting application shutdown after job completion.")
				
				if err := shutdowner.Shutdown(); err != nil {
					logger.Errorf("Failed to shutdown application: %v", err)
				}
			}()

			jobName := cfg.Surfin.Batch.JobName
			logger.Infof("Starting actual job execution for job '%s'...", jobName)

			jobParams := model.NewJobParameters()

			jobExecution, err := jobLauncher.Launch(appCtx, jobName, jobParams)
			if err != nil {
				logger.Errorf("Failed to launch job '%s': %v", jobName, err)
				return
			}
			logger.Infof("Job '%s' launched successfully. Execution ID: %s", jobName, jobExecution.ID)

			pollingInterval := time.Duration(cfg.Surfin.Batch.PollingIntervalSeconds) * time.Second
			if pollingInterval == 0 {
				pollingInterval = 5 * time.Second
			}
			logger.Infof("Monitoring job '%s' (Execution ID: %s) with polling interval %v...", jobName, jobExecution.ID, pollingInterval)

			for {
				select {
				case <-ctx.Done():
					logger.Warnf("Application context cancelled. Stopping monitoring for job '%s' (Execution ID: %s).", jobName, jobExecution.ID)
					
					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(context.Background(), jobExecution.ID)
					if fetchErr == nil && !latestExecution.Status.IsFinished() {
						logger.Warnf("Job '%s' (Execution ID: %s) was running. Attempting graceful stop via JobOperator.", jobName, jobExecution.ID)
						if cancelFunc, ok := jobLauncher.GetCancelFunc(jobExecution.ID); ok {
							cancelFunc()
						}
					}
					return
				case <-time.After(pollingInterval):
					latestExecution, fetchErr := jobRepository.FindJobExecutionByID(ctx, jobExecution.ID)
					if fetchErr != nil {
						logger.Errorf("Failed to fetch latest status for JobExecution (ID: %s): %v", jobExecution.ID, fetchErr)
						continue
					}

					if latestExecution.Status.IsFinished() {
						logger.Infof("Job '%s' (Execution ID: %s) finished with status: %s, ExitStatus: %s",
							jobName, latestExecution.ID, latestExecution.Status, latestExecution.ExitStatus)
						
						return
					}
					logger.Debugf("Job '%s' (Execution ID: %s) is still running. Current status: %s", jobName, latestExecution.ID, latestExecution.Status)
				}
			}
		}()
		return nil
	}
}

// onStopApplication はアプリケーションシャットダウンをログに記録する Fx Hook ヘルパー関数です。
func onStopApplication() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Application is shutting down.")
		return nil
	}
}

// main はアプリケーションのエントリポイントです。
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナルハンドリング (Ctrl+Cなど)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal '%v'. Attempting to stop the job...", sig)
		cancel()
	}()

	envFilePath := os.Getenv("ENV_FILE_PATH")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	// GetApplicationOptions から返されたオプションを展開して fx.New に渡す
	fxApp := fx.New(GetApplicationOptions(ctx, envFilePath, embeddedConfig, embeddedJSL)...) // GetApplicationOptions から返されたオプションを展開して fx.New に渡す
	fxApp.Run()
	if fxApp.Err() != nil {
		logger.Fatalf("Application run failed: %v", fxApp.Err())
	}
	os.Exit(0)
}
```

**内容のポイント:**
*   `embeddedConfig`、`embeddedJSL`は、それぞれ設定ファイル、JSLファイルをアプリケーションに埋め込みます。
*   `main`関数は、シグナルハンドリングを設定し、`fx.New`を呼び出してFxアプリケーションを起動します。

### 6. バッチアプリケーションの実行

これで、データベースを使用しない「Hello, World!」バッチを実行する準備ができました。

`example/hello-world/Taskfile.yaml` を開き、`run`タスクの`cmds`セクションを確認してください。

```yaml # example/hello-world/Taskfile.yaml
version: '3'

vars:
  APP_MODULE_PATH: surfin/example/hello-world/cmd/hello-world
  APP_BINARY_NAME: hello-world
  BUILD_OUTPUT_DIR: ../../dist

tasks:
  default:
    desc: "List tasks for the hello-world application."
    cmds:
      - task --list

  build:
    desc: "Build the hello-world application executable."
    cmds:
      - go build -v -gcflags="all=-N -l" -o {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}} {{.APP_MODULE_PATH}}
      - echo "Built {{.APP_BINARY_NAME}} to {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"
    generates:
      - "{{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"
    sources:
      - "./cmd/hello-world/**/*.go"
      - "./internal/**/*.go"
      - "./cmd/hello-world/resources/application.yaml"
      - "./cmd/hello-world/resources/job.yaml"

  run:
    desc: "Run the hello-world application."
    deps: [build]
    cmds: ["{{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"]
    env:
      BATCH_LOG_LEVEL: DEBUG
      # ENV_FILE_PATH: .env

  clean:
    desc: "Remove build artifacts for hello-world application."
    cmds:
      - rm -f {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}

  test:
    desc: "Run tests for the hello-world application."
    cmds:
      - go test ./internal/... -v -count=1
```

**内容のポイント:**

*   `run`コマンドは、`example/hello-world` ディレクトリ内で`task run`を実行することを想定しており、`cd ../../`のようなディレクトリ移動は不要です。

次に、ターミナルで`example/hello-world` ディレクトリに移動し、以下のコマンドを実行します。

```bash
task build
task run
```

### 7. 実行結果の確認

アプリケーションが起動し、ログの中に以下のような行が表示されるはずです。

```
...
WARN[xxxx-xx-xx xx:xx:xx] Running in DB-less mode. No DB providers will be registered.
WARN[xxxx-xx-xx xx:xx:xx] Running in DB-less mode. Providing dummy DB connections and transaction managers.
WARN[xxxx-xx-xx xx:xx:xx] Running in DB-less mode. Providing dummy DB connection resolver.
INFO[xxxx-xx-xx xx:xx:xx] HelloWorldTasklet: Hello, Surfin Batch World!
INFO[xxxx-xx-xx xx:xx:xx] Job 'helloWorldJob' launched successfully. Execution ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
INFO[xxxx-xx-xx xx:xx:xx] Monitoring job 'helloWorldJob' (Execution ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) with polling interval 3600s...
INFO[xxxx-xx-xx xx:xx:xx] Job 'helloWorldJob' (Execution ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) finished with status: COMPLETED, ExitStatus: COMPLETED
INFO[xxxx-xx-xx xx:xx:xx] Application is shutting down.
...
```

`WARN`ログは、データベース関連のコンポーネントがダミー実装に置き換えられたことを示しています。これにより、実際のデータベース接続なしでバッチが正常に実行され、「Hello, Surfin Batch World!」が出力されることを確認できます。

---

### まとめ

このチュートリアルを通じて、Surfin Batch Framework の基本的なコンポーネントの連携、カスタムロジックの実装、そしてデータベースに依存しないシンプルなバッチアプリケーションの構築方法を学びました。

*   JSL を使ってジョブのフローを定義する方法。
*   `Tasklet` を実装し、カスタムロジックを組み込む方法。
*   Fx を使ってカスタムコンポーネントをフレームワークに登録する方法。
*   `JobRepository` や DB 関連コンポーネントをダミー実装に置き換えることで、DBレス環境でバッチを実行する方法。

この知識を基に、より複雑なバッチ処理を構築するための第一歩を踏み出せます。
