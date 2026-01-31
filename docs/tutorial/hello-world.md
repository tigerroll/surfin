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
mkdir -p example/hello-world/internal/app/job
mkdir -p example/hello-world/internal/app/runner
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
    logging: # Set log level to DEBUG
      level: DEBUG
  batch:
    job_name: helloWorldJob
    polling_interval_seconds: 5
    chunk_size: 1000
    item_retry:
      max_attempts: 3
      initial_interval: 100
      retryable_exceptions: # Retryable exceptions for item processing
        - "BatchError"
    item_skip:
      skip_limit: 10
      skippable_exceptions: # Skippable exceptions for item processing
        - "BatchError" # No need for datasources and infrastructure sections for in-memory DB
    job_repository_db_ref: dummy
  database: # Renamed from adapter_configs to database
    metadata: # Dummy configuration for 'metadata' database, referenced by framework migrations
      type: dummy
  security:
    masked_parameter_keys:
      - "password"
      - "api_key"
```

#### `example/hello-world/Taskfile.yaml`

```yaml
version: '3'

vars:
  APP_MODULE_PATH: ./cmd/hello-world
  APP_BINARY_NAME: hello-world
  BUILD_OUTPUT_DIR: ../../dist

tasks:
  default:
    desc: "List tasks for the hello-world application."
    cmds: [task --list]

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
    cmds: ["rm -f {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"]

  test:
    desc: "Run tests for the hello-world application."
    cmds: ["go test ./internal/... -v -count=1"]
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
	configbinder "github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// HelloWorldTaskletConfig は JSL から渡されるプロパティをバインドするための構造体です。
// It defines the configuration parameters for the HelloWorldTasklet.
type HelloWorldTaskletConfig struct {
	Message string `yaml:"message"` // Corresponds to properties.message in JSL.
}

// HelloWorldTasklet はシンプルなTaskletの実装です。
// It prints a configurable message to the console.
type HelloWorldTasklet struct {
	config           *HelloWorldTaskletConfig // Configuration for the tasklet.
	executionContext model.ExecutionContext   // ExecutionContext to hold the internal state of the Tasklet.
}

// NewHelloWorldTasklet は HelloWorldTasklet の新しいインスタンスを作成します。
// It binds the provided properties to the tasklet's configuration.
func NewHelloWorldTasklet(properties map[string]string) (*HelloWorldTasklet, error) {
	taskletCfg := &HelloWorldTaskletConfig{}

	if err := configbinder.BindProperties(properties, taskletCfg); err != nil { // Binds JSL properties to the struct.
		// isSkippable and isRetryable are set to false.
		return nil, exception.NewBatchError("hello_world_tasklet", "Failed to bind properties", err, false, false)
	}

	if taskletCfg.Message == "" {
		return nil, fmt.Errorf("message property is required for HelloWorldTasklet")
	}

	return &HelloWorldTasklet{
		config:           taskletCfg,
		executionContext: model.NewExecutionContext(),
	}, nil
}

// Execute runs the main business logic of the Tasklet.
// It logs the configured message to the console.
func (t *HelloWorldTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return model.ExitStatusFailed, ctx.Err()
	default:
	}
	// Add a debug log to confirm the message content.
	logger.Debugf("HelloWorldTasklet: Attempting to log message: '%s'", t.config.Message) // Log the message being processed.
	logger.Infof("HelloWorldTasklet: %s", t.config.Message)
	return model.ExitStatusCompleted, nil
}

// Close releases any resources held by the Tasklet.
// For HelloWorldTasklet, there are no specific resources to close.
func (t *HelloWorldTasklet) Close(ctx context.Context) error {
	logger.Debugf("HelloWorldTasklet: Close called.")
	return nil
}

// SetExecutionContext sets the ExecutionContext for the Tasklet.
// This method allows the framework to inject or restore the tasklet's state.
func (t *HelloWorldTasklet) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	t.executionContext = ec
	return nil
}

// GetExecutionContext retrieves the current ExecutionContext of the Tasklet.
// This method allows the framework to persist the tasklet's state.
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
	config "github.com/tigerroll/surfin/pkg/batch/core/config" // Import config package
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	support "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// NewHelloWorldTaskletComponentBuilder creates a jsl.ComponentBuilder for HelloWorldTasklet.
// This builder function is responsible for instantiating HelloWorldTasklet
// with its required properties.
func NewHelloWorldTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		dbResolver core.DBConnectionResolver,
		properties map[string]string,
	) (interface{}, error) {
		// Unused arguments are ignored for this component.
		_ = cfg
		_ = resolver
		_ = dbResolver

		tasklet, err := NewHelloWorldTasklet(properties)
		if err != nil {
			return nil, err
		}
		return tasklet, nil
	})
}

// RegisterHelloWorldTaskletBuilder registers the created ComponentBuilder with the JobFactory.
//
// This allows the framework to locate and use the HelloWorldTasklet
// when it's referenced in JSL (Job Specification Language) files.
func RegisterHelloWorldTaskletBuilder(
	jf *support.JobFactory,
	builder jsl.ComponentBuilder,
) { // Register the ComponentBuilder with the JobFactory.
	// The key "helloWorldTasklet" must match the 'ref' attribute in the JSL (e.g., job.yaml).
	jf.RegisterComponentBuilder("helloWorldTasklet", builder)
	logger.Debugf("ComponentBuilder for HelloWorldTasklet registered with JobFactory. JSL ref: 'helloWorldTasklet'")
}

// Module defines the Fx options for the HelloWorldTasklet component.
// It provides the component builder and registers it with the JobFactory.
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
	// Standard library imports
	"context"

	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
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
		id:             "helloWorldJob", // Matches JSL ID
		name:           "Hello World Batch Job",
		flow:           flow,
		jobRepository:  jobRepository,
		cfg:            cfg,
		listeners:      listeners,
		metricRecorder: metricRecorder,
		tracer:         tracer,
	}, nil
}

// Run contains the job's execution logic.
// The SimpleJobLauncher calls this Run method using a JobRunner.
// Therefore, this method does not directly execute the flow but delegates
// the processing to the JobRunner. This specific Job implementation
// does not directly reference the JobRunner, so it performs no operation
// or just logs.
func (j *HelloWorldJob) Run(ctx context.Context, jobExecution *model.JobExecution, jobParameters model.JobParameters) error {
	logger.Infof("HelloWorldJob.Run called for JobExecution ID: %s", jobExecution.ID)
	// The actual flow execution is handled by the JobRunner.
	return nil
}

// JobName returns the logical name of the job.
func (j *HelloWorldJob) JobName() string {
	return j.name
}

// ID returns the unique ID of the job definition.
func (j *HelloWorldJob) ID() string {
	return j.id
}

// GetFlow returns the job's flow definition structure.
func (j *HelloWorldJob) GetFlow() *model.FlowDefinition {
	return j.flow
}

// ValidateParameters validates job parameters before job execution.
func (j *HelloWorldJob) ValidateParameters(params model.JobParameters) error {
	// Parameter validation is not needed for this tutorial.
	return nil
}

// HelloWorldJob confirms that it implements the port.Job interface.
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
// It ensures that the "helloWorldJob" can be instantiated by the framework
// when referenced in JSL (Job Specification Language) files.
func RegisterHelloWorldJobBuilder(
	jf *support.JobFactory,
	builder support.JobBuilder,
) {
	// Register the JobBuilder with the JobFactory using the key "helloWorldJob".
	// This key must match the 'id' field in the JSL (e.g., job.yaml).
	jf.RegisterJobBuilder("helloWorldJob", builder)
	logger.Debugf("JobBuilder for helloWorldJob registered with JobFactory. JSL id: 'helloWorldJob'")
}

// provideHelloWorldJobBuilder は NewHelloWorldJob 関数を support.JobBuilder 型として提供します。
// The dependencies of NewHelloWorldJob are resolved when the JobBuilder returned by this function
// is actually invoked by the framework.
func provideHelloWorldJobBuilder() support.JobBuilder {
	return NewHelloWorldJob
}

// Module は helloWorldJob コンポーネントの Fx オプションを定義します。
var Module = fx.Options(
	fx.Provide(fx.Annotate(
		provideHelloWorldJobBuilder,           // The provideHelloWorldJobBuilder function returns a support.JobBuilder type.
		fx.ResultTags(`name:"helloWorldJob"`), // Tags the result so JobFactory can retrieve the JobBuilder by this name.
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
	// Standard library imports
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

	// Update JobExecution status to STARTED.
	jobExecution.MarkAsStarted() // Mark the job execution as started.
	if err := r.jobRepository.UpdateJobExecution(ctx, jobExecution); err != nil {
		logger.Errorf("FlowJobRunner: Failed to update JobExecution status to STARTED: %v", err)
		jobExecution.MarkAsFailed(err)
		r.jobRepository.UpdateJobExecution(ctx, jobExecution) // Attempt to save the final status.
		return
	}

	// Start a tracing span for job execution.
	jobCtx, endJobSpan := r.tracer.StartJobSpan(ctx, jobExecution)
	defer endJobSpan()

	// Get the starting element from the flow definition.
	currentElementID := flowDef.StartElement
	var currentElement interface{}
	var ok bool

	for {
		select {
		case <-jobCtx.Done():
			logger.Warnf("FlowJobRunner: Job context cancelled for Job '%s' (Execution ID: %s).", jobInstance.JobName(), jobExecution.ID) // Log cancellation.
			jobExecution.MarkAsStopped()
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		default:
			// Continue
		}

		currentElement, ok = flowDef.Elements[currentElementID]
		if !ok { // Check if the current element exists in the flow definition.
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
			logger.Infof("FlowJobRunner: Executing Step '%s' for Job '%s'.", element.StepName(), jobInstance.JobName()) // Log step execution.

			// Create a new StepExecution.
			stepExecution := model.NewStepExecution(model.NewID(), jobExecution, element.StepName())
			jobExecution.AddStepExecution(stepExecution)      // Add to the list of StepExecutions for the JobExecution.
			jobExecution.CurrentStepName = element.StepName() // Update the current step name.

			// Save the StepExecution initially (workaround if SimpleStepExecutor doesn't save).
			// Although StepExecutor should handle this within a transaction, the current implementation
			// might be lacking, so this compensates. This ensures that the first UpdateStepExecution
			// call within TaskletStep/ChunkStep succeeds.
			if err := r.jobRepository.SaveStepExecution(jobCtx, stepExecution); err != nil {
				elementErr = exception.NewBatchError(element.StepName(), "Failed to save initial StepExecution", err, false, false)
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Failed to save initial StepExecution for Step '%s': %v", element.StepName(), err)
				jobExecution.MarkAsFailed(elementErr)
				r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
				return // Exit the Run method.
			}
			// Execute the step.
			executedStepExecution, err := r.stepExecutor.ExecuteStep(jobCtx, element, jobExecution, stepExecution)
			if err != nil {
				elementErr = err
				exitStatus = model.ExitStatusFailed
				logger.Errorf("FlowJobRunner: Step '%s' failed: %v", element.StepName(), err)
			} else {
				exitStatus = executedStepExecution.ExitStatus
				logger.Infof("FlowJobRunner: Step '%s' completed with ExitStatus: %s", element.StepName(), exitStatus)
			}

			// Promote ExecutionContext from Step to Job.
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
			// Determine the next path based on the decision.
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
			// TODO: Implement parallel execution for Split.
			// Currently, it returns an error as it's not yet implemented.
			elementErr = exception.NewBatchErrorf("flow_runner", "Split execution is not yet implemented for Split '%s'", element.ID())
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)

		default:
			elementErr = exception.NewBatchErrorf("flow_runner", "Unknown flow element type for ID '%s': %T", currentElementID, currentElement)
			exitStatus = model.ExitStatusFailed
			logger.Errorf("FlowJobRunner: %v", elementErr)
		}

		// Search for the next transition rule.
		nextRule, found := flowDef.GetTransitionRule(currentElementID, exitStatus, elementErr != nil)
		if !found {
			// If a specific rule is not found, try a wildcard or default rule.
			nextRule, found = flowDef.GetTransitionRule(currentElementID, model.ExitStatusUnknown, elementErr != nil) // Check for '*'
		}

		if !found { // If no transition rule is found, the job terminates as failed.
			err := exception.NewBatchErrorf("flow_runner", "No transition rule found for element '%s' with ExitStatus '%s' (error: %v)", currentElementID, exitStatus, elementErr)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err)
			r.jobRepository.UpdateJobExecution(jobCtx, jobExecution)
			return
		}

		// Apply the transition rule.
		if nextRule.Transition.End {
			jobExecution.MarkAsCompleted()
			if elementErr != nil { // If there was an error but the transition is 'end', still mark as completed.
				jobExecution.AddFailureException(elementErr)
			}
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) completed with ExitStatus: %s (Transition: END).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.Fail {
			jobExecution.MarkAsFailed(elementErr)
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) failed with ExitStatus: %s (Transition: FAIL).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.Stop {
			jobExecution.MarkAsStopped()
			logger.Infof("FlowJobRunner: Job '%s' (Execution ID: %s) stopped with ExitStatus: %s (Transition: STOP).", jobInstance.JobName(), jobExecution.ID, jobExecution.ExitStatus)
			break // Exit the loop.
		} else if nextRule.Transition.To != "" {
			currentElementID = nextRule.Transition.To
			logger.Debugf("FlowJobRunner: Transitioning to next element: '%s'", currentElementID)
		} else {
			// This should not happen if validation is correct, but as a safeguard.
			err := exception.NewBatchErrorf("flow_runner", "Invalid transition rule for element '%s': no 'to', 'end', 'fail', or 'stop' specified", currentElementID)
			logger.Errorf("FlowJobRunner: %v", err)
			jobExecution.MarkAsFailed(err) // Mark job as failed.
			break                          // Exit the loop.
		}
	}

	// Final update of JobExecution (if not already updated by a break condition).
	if !jobExecution.Status.IsFinished() { // If the loop ends without an explicit final status, consider it completed.
		jobExecution.MarkAsCompleted()
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
	"database/sql"
	"io/fs"

	helloTasklet "github.com/tigerroll/surfin/example/hello-world/internal/step"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	dummy "github.com/tigerroll/surfin/pkg/batch/adapter/database/dummy"
	// Batch framework imports
	item "github.com/tigerroll/surfin/pkg/batch/component/item"
	migration "github.com/tigerroll/surfin/pkg/batch/component/tasklet/migration"
	adapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	bootstrap "github.com/tigerroll/surfin/pkg/batch/core/config/bootstrap"
	jsl "github.com/tigerroll/surfin/pkg/batch/core/config/jsl"
	supportConfig "github.com/tigerroll/surfin/pkg/batch/core/config/support"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	decision "github.com/tigerroll/surfin/pkg/batch/core/job/decision"
	split "github.com/tigerroll/surfin/pkg/batch/core/job/split"
	metrics "github.com/tigerroll/surfin/pkg/batch/core/metrics"
	incrementer "github.com/tigerroll/surfin/pkg/batch/core/support/incrementer"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	inmemoryRepo "github.com/tigerroll/surfin/pkg/batch/infrastructure/repository/inmemory"
	batchlistener "github.com/tigerroll/surfin/pkg/batch/listener"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"

	appjob "github.com/tigerroll/surfin/example/hello-world/internal/app/job"
	apprunner "github.com/tigerroll/surfin/example/hello-world/internal/app/runner"
)

// dummyMigrator is a dummy implementation of the migration.Migrator interface.
// It performs no actual migration operations, as the hello-world application
// does not require real database migrations.
type dummyMigrator struct{}

func (d *dummyMigrator) Up(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Up called, doing nothing.")
	return nil
}
func (d *dummyMigrator) Down(ctx context.Context, fsys fs.FS, dir, table string) error {
	logger.Debugf("Dummy Migrator: Down called, doing nothing.")
	return nil
}
func (d *dummyMigrator) Close() error {
	logger.Debugf("Dummy Migrator: Close called, doing nothing.")
	return nil
}

// dummyMigratorProvider is a dummy implementation of the migration.MigratorProvider interface.
// It provides dummy Migrator instances, as real migrations are not needed
// for the hello-world application.
type dummyMigratorProvider struct{}

func (d *dummyMigratorProvider) NewMigrator(conn adapter.DBConnection) migration.Migrator {
	return &dummyMigrator{}
}

// dummyDBConnection is a dummy implementation of the adapter.DBConnection interface.
// It performs no actual database operations, as the hello-world application
// runs in a DB-less mode.
type dummyDBConnection struct{}

// ExecuteUpdate is a dummy implementation of the DBExecutor.ExecuteUpdate method.
func (d *dummyDBConnection) ExecuteUpdate(ctx context.Context, model interface{}, operation string, tableName string, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpdate called, doing nothing.")
	return 0, nil
}

// ExecuteUpsert is a dummy implementation of the DBExecutor.ExecuteUpsert method.
func (d *dummyDBConnection) ExecuteUpsert(ctx context.Context, model interface{}, tableName string, uniqueColumns []string, updateColumns []string) (int64, error) {
	logger.Debugf("Dummy DBConnection: ExecuteUpsert called, doing nothing. Table: %s", tableName)
	return 0, nil
}

// ExecuteQuery is a dummy implementation of the DBExecutor.ExecuteQuery method.
func (d *dummyDBConnection) ExecuteQuery(ctx context.Context, model interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: ExecuteQuery called, doing nothing. Query: %v", query)
	return nil
}

// Count is a dummy implementation of the DBExecutor.Count method.
func (d *dummyDBConnection) Count(ctx context.Context, model interface{}, query map[string]interface{}) (int64, error) {
	logger.Debugf("Dummy DBConnection: Count called, doing nothing. Query: %v", query)
	return 0, nil
}

// ExecuteQueryAdvanced is a dummy implementation of the DBExecutor.ExecuteQueryAdvanced method.
func (d *dummyDBConnection) ExecuteQueryAdvanced(ctx context.Context, model interface{}, query map[string]interface{}, orderBy string, limit int) error {
	logger.Debugf("Dummy DBConnection: ExecuteQueryAdvanced called, doing nothing. Query: %v, OrderBy: %s, Limit: %d", query, orderBy, limit)
	return nil
}

// Pluck is a dummy implementation of the DBExecutor.Pluck method.
func (d *dummyDBConnection) Pluck(ctx context.Context, dest interface{}, field string, value interface{}, query map[string]interface{}) error {
	logger.Debugf("Dummy DBConnection: Pluck called, doing nothing. Field: %s, Value: %v, Query: %v", field, value, query)
	return nil
}

// RefreshConnection is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) RefreshConnection(ctx context.Context) error {
	logger.Debugf("Dummy DBConnection: RefreshConnection called, doing nothing.")
	return nil
}

// Type returns the type of the dummy database connection.
func (d *dummyDBConnection) Type() string { return "dummy" }

// Name returns the name of the dummy database connection.
func (d *dummyDBConnection) Name() string { return "dummy" }

// Close closes the dummy database connection.
func (d *dummyDBConnection) Close() error { return nil }

// IsTableNotExistError checks if the given error indicates that a table does not exist (always false for dummy).
func (d *dummyDBConnection) IsTableNotExistError(err error) bool { return false }

// Config is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) Config() dbconfig.DatabaseConfig { return dbconfig.DatabaseConfig{} }

// GetSQLDB is a dummy implementation of the DBConnection interface.
func (d *dummyDBConnection) GetSQLDB() (*sql.DB, error) {
	return nil, nil // Returns nil as there's no actual SQL DB.
}

// dummyDBProvider is a dummy implementation of the adapter.DBProvider interface.
// It always returns dummy DBConnection instances.
type dummyDBProvider struct{}

// GetConnection returns a dummy DBConnection.
func (d *dummyDBProvider) GetConnection(name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: GetConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// ForceReconnect returns a new dummy DBConnection, simulating a re-establishment.
func (d *dummyDBProvider) ForceReconnect(name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy DBProvider: ForceReconnect called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// CloseAll performs no operation for dummy connections.
func (d *dummyDBProvider) CloseAll() error {
	logger.Debugf("Dummy DBProvider: CloseAll called.")
	return nil
}

// Type returns the type of the dummy database provider.
func (d *dummyDBProvider) Type() string { return "dummy" }

// dummyPortDBConnectionResolver is a dummy implementation of the port.DBConnectionResolver interface.
// It's used to satisfy dependencies in a DB-less environment.
type dummyPortDBConnectionResolver struct{}

// ResolveDBConnectionName returns the default connection name for dummy operations.
func (d *dummyPortDBConnectionResolver) ResolveDBConnectionName(ctx context.Context, jobExecution *model.JobExecution, stepExecution *model.StepExecution, defaultName string) (string, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnectionName called, returning default '%s'.", defaultName)
	return defaultName, nil
}

// ResolveDBConnection returns a dummy DBConnection instance.
func (d *dummyPortDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy Port DBConnectionResolver: ResolveDBConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// dummyAdapterDBConnectionResolver is a dummy implementation of the adapter.DBConnectionResolver interface.
// It's used to satisfy dependencies in a DB-less environment.
type dummyAdapterDBConnectionResolver struct{}

// ResolveDBConnection returns a dummy DBConnection instance.
func (d *dummyAdapterDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adapter.DBConnection, error) {
	logger.Debugf("Dummy Adapter DBConnectionResolver: ResolveDBConnection called for '%s'.", name)
	return &dummyDBConnection{}, nil
}

// GetApplicationOptions constructs and returns a slice of uber-fx options.
// This function must be defined before the fx.New call.
func GetApplicationOptions(appCtx context.Context, envFilePath string, embeddedConfig config.EmbeddedConfig, embeddedJSL jsl.JSLDefinitionBytes) []fx.Option {
	cfg, err := config.LoadConfig(envFilePath, embeddedConfig) // Load application configuration.
	if err != nil {                                            // Fatal error if configuration loading fails.
		logger.Fatalf("Failed to load configuration: %v", err) // Log and exit if config loading fails.
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
	// Dummy providers to satisfy framework migration dependencies.
	options = append(options, fx.Provide(func() migration.MigratorProvider { return &dummyMigratorProvider{} }))
	options = append(options, fx.Provide(fx.Annotate(
		func() map[string]fs.FS { return make(map[string]fs.FS) },
		fx.ResultTags(`name:"allMigrationFS"`),
	)))
	// Add dummy providers for missing DB-related dependencies.
	options = append(options, fx.Provide(func() port.DBConnectionResolver { return &dummyPortDBConnectionResolver{} }))       // Provides port.DBConnectionResolver.
	options = append(options, fx.Provide(func() adapter.DBConnectionResolver { return &dummyAdapterDBConnectionResolver{} })) // Provides adapter.DBConnectionResolver.
	options = append(options, fx.Provide(func() map[string]adapter.DBProvider {
		return map[string]adapter.DBProvider{
			"default":  &dummyDBProvider{}, // Provides at least one dummy provider.
			"metadata": &dummyDBProvider{}, // Adds dummy provider for "metadata" database for framework migrations.
			"dummy":    &dummyDBProvider{}, // Adds dummy provider for "dummy" database for JobRepositoryDBRef.
		}
	}))
	// Add dummy providers for missing transaction manager-related dependencies.
	options = append(options, fx.Provide(func() tx.TransactionManagerFactory {
		return &dummy.DummyTxManagerFactory{}
	}))
	options = append(options, fx.Provide(func() map[string]tx.TransactionManager {
		return make(map[string]tx.TransactionManager) // Provides an empty map to satisfy NewMetadataTxManager's dependencies.
	}))
	options = append(options, fx.Provide(fx.Annotate(dummy.NewMetadataTxManager, fx.ResultTags(`name:"metadata"`))))

	options = append(options, logger.Module)
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
	options = append(options, helloTasklet.Module) // Include the module for HelloWorldTasklet.
	options = append(options, appjob.Module)       // Directly include the module that provides application-specific JobBuilders.

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

	usecase "github.com/tigerroll/surfin/pkg/batch/core/application/usecase"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	jobRepo "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"go.uber.org/fx"
)

// embeddedConfig embeds the content of the application's YAML configuration file.
//
//go:embed resources/application.yaml
var embeddedConfig []byte

// embeddedJSL embeds the content of the Job Specification Language (JSL) file.
// This file defines the batch job's structure and components.
//
//go:embed resources/job.yaml
var embeddedJSL []byte

// startJobExecution is an Fx Hook helper function that initiates job execution
// upon application startup. It registers OnStart and OnStop hooks with the Fx lifecycle.
func startJobExecution(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	// jobLauncher is the concrete SimpleJobLauncher instance responsible for launching jobs.
	jobLauncher *usecase.SimpleJobLauncher,
	jobRepository jobRepo.JobRepository,
	cfg *config.Config,
	appCtx context.Context,
) {
	lc.Append(fx.Hook{
		OnStart: onStartJobExecution(jobLauncher, jobRepository, cfg, shutdowner, appCtx),
		OnStop:  onStopApplication(),
	})
}

// onStartJobExecution is an Fx Hook helper function that returns a function
// to be executed when the application starts. It launches the batch job
// and monitors its execution, triggering application shutdown upon completion.
func onStartJobExecution(
	// jobLauncher is the concrete SimpleJobLauncher instance responsible for launching jobs.
	jobLauncher *usecase.SimpleJobLauncher,
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

// onStopApplication is an Fx Hook helper function that returns a function
// to be executed when the application stops. It logs the application shutdown event.
func onStopApplication() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Infof("Application is shutting down.")
		return nil
	}
}

// main is the entry point of the hello-world batch application.
// It sets up the application context, handles OS signals for graceful shutdown,
// loads configuration, and initializes and runs the Fx application.
//
// The application will execute the "helloWorldJob" defined in job.yaml.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals (e.g., Ctrl+C) for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Warnf("Received signal '%v'. Attempting to stop the job...", sig)
		cancel()
	}()

	// Determine the path to the .env file, defaulting to ".env" if not specified.
	envFilePath := os.Getenv("ENV_FILE_PATH")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	fxApp := fx.New(GetApplicationOptions(ctx, envFilePath, embeddedConfig, embeddedJSL)...)
	fxApp.Run()
	if fxApp.Err() != nil { // Check for errors during Fx application startup or execution.
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
  APP_MODULE_PATH: ./cmd/hello-world
  APP_BINARY_NAME: hello-world
  BUILD_OUTPUT_DIR: ../../dist

tasks:
  default:
    desc: "List tasks for the hello-world application."
    cmds: [task --list]

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
    cmds: ["rm -f {{.BUILD_OUTPUT_DIR}}/{{.APP_BINARY_NAME}}"]

  test:
    desc: "Run tests for the hello-world application."
    cmds: ["go test ./internal/... -v -count=1"]
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
