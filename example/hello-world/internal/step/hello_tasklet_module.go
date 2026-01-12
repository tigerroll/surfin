package step

import (
	"go.uber.org/fx"

	core "surfin/pkg/batch/core/application/port"
	config "surfin/pkg/batch/core/config"
	jsl "surfin/pkg/batch/core/config/jsl"
	support "surfin/pkg/batch/core/config/support"
	job "surfin/pkg/batch/core/domain/repository"
	logger "surfin/pkg/batch/support/util/logger" // この行を追加
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
	logger.Debugf("ComponentBuilder for HelloWorldTasklet registered with JobFactory. JSL ref: 'helloWorldTasklet'") // この行を追加
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
