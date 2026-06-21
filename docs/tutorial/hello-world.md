# 👉 チュートリアル - "Hello, World!"

このチュートリアルでは、Surfin Batch Framework を使用して、コンソールにメッセージを出力するシンプルなバッチアプリケーションを構築・拡張します。

「まずは動かし、仕組みを理解し、最後に自分で機能を追加する」というステップで進めます。

---

### Step 1: Hello World を動かそう

まずは、すでに用意されている `hello-world` アプリケーションを動かしてみましょう。

1.  **ディレクトリ移動**:
    ```bash
    cd example/hello-world
    ```

2.  **実行**:
    ```bash
    task run
    ```

3.  **確認**:
    ログの最後に `HelloWorldTasklet: Hello, Surfin Batch World!` と出力されていれば成功です。

---

### Step 2: 仕組みを覗こう

なぜこのコードが動くのか、重要なポイントを解説します。

*   **DI (Fx) の役割 (`cmd/hello-world/app_options.go`)**:
    アプリケーションの起動時に、ダミーのデータベースやリソースを注入しています。フレームワークの複雑な設定を隠蔽し、ビジネスロジックに集中できるようにしています。
*   **JSL (YAML) の役割 (`cmd/hello-world/resources/job.yaml`)**:
    ジョブの設計図です。`helloWorldStep` というステップが定義されており、`helloWorldTasklet` という名前のコンポーネントを呼び出しています。
*   **Tasklet の役割 (`internal/step/hello_tasklet.go`)**:
    実際の処理本体です。`port.Tasklet` インターフェースを実装しており、`Execute` メソッドが呼ばれると、設定されたメッセージをログに出力します。

---

### Step 3: 改造してみよう (Hands-on)

既存の `HelloWorldTasklet` を参考に、新しい `GoodbyeTasklet` を作成し、ジョブフローに追加してみましょう。

#### 1. GoodbyeTasklet の作成
`internal/step/goodbye_tasklet.go` を新規作成します。

```go
package step

import (
	"context"
	"github.com/tigerroll/surfin/pkg/batch/core/application/port"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

type GoodbyeTasklet struct{}

func NewGoodbyeTasklet() (*GoodbyeTasklet, error) {
	return &GoodbyeTasklet{}, nil
}

func (t *GoodbyeTasklet) StepName() string { return "goodbyeStep" }
func (t *GoodbyeTasklet) GetExecutionContextPromotion() *model.ExecutionContextPromotion { return nil }
func (t *GoodbyeTasklet) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	logger.Infof("GoodbyeTasklet: Goodbye, Surfin Batch World!")
	return model.ExitStatusCompleted, nil
}
func (t *GoodbyeTasklet) Open(ctx context.Context, stepExecution *model.StepExecution) error { return nil }
func (t *GoodbyeTasklet) Close(ctx context.Context, stepExecution *model.StepExecution) error { return nil }
func (t *GoodbyeTasklet) SetExecutionContext(ec model.ExecutionContext) {}
func (t *GoodbyeTasklet) GetExecutionContext() model.ExecutionContext { return model.NewExecutionContext() }

var _ port.Tasklet = (*GoodbyeTasklet)(nil)
```

#### 2. モジュールへの登録
`internal/step/hello_tasklet_module.go` を編集し、新しい Tasklet を登録します。

```go
// (既存のコードの下に以下を追加)

func NewGoodbyeTaskletComponentBuilder() jsl.ComponentBuilder {
	return jsl.ComponentBuilder(func(
		cfg *config.Config,
		resolver core.ExpressionResolver,
		resourceProviders map[string]coreAdapter.ResourceProvider,
		properties map[string]interface{},
	) (interface{}, error) {
		return NewGoodbyeTasklet()
	})
}

func RegisterGoodbyeTaskletBuilder(jf *support.JobFactory, builder jsl.ComponentBuilder) {
	jf.RegisterComponentBuilder("goodbyeTasklet", builder)
}

// Module の fx.Options に以下を追加してください:
// fx.Provide(fx.Annotate(NewGoodbyeTaskletComponentBuilder, fx.ResultTags(`name:"goodbyeTasklet"`))),
// fx.Invoke(fx.Annotate(RegisterGoodbyeTaskletBuilder, fx.ParamTags(``, `name:"goodbyeTasklet"`))),
```

#### 3. ジョブフローの更新
`cmd/hello-world/resources/job.yaml` を編集し、`helloWorldStep` の後に `goodbyeStep` を実行するように変更します。

```yaml
flow:
  start-element: helloWorldStep
  elements:
    helloWorldStep:
      # ... (既存の設定)
      transitions:
        - on: COMPLETED
          to: goodbyeStep # 次のステップへ
    goodbyeStep:
      id: goodbyeStep
      tasklet:
        ref: goodbyeTasklet
      transitions:
        - on: COMPLETED
          end: true
```

---

### まとめ

この構成を理解すれば、以下の手順でバッチ処理を拡張できることがわかります。

1.  **Tasklet を作る**: `port.Tasklet` インターフェースを実装する。
2.  **DI に登録する**: `JobFactory` に名前を付けて登録する。
3.  **YAML で繋ぐ**: `job.yaml` でステップを定義し、フローを構築する。

これで、Surfin Batch Framework を使った開発の基本はマスターしました！
