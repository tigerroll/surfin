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
type HelloWorldTaskletConfig struct {
	Message string `yaml:"message"` // JSLのproperties.messageに対応
}

// HelloWorldTasklet はシンプルなTaskletの実装です。
type HelloWorldTasklet struct {
	config           *HelloWorldTaskletConfig
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
		config:           taskletCfg,
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

	// デバッグログを追加して、メッセージの内容を確認
	logger.Debugf("HelloWorldTasklet: Attempting to log message: '%s'", t.config.Message)
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
