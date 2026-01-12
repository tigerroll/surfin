package job

import "go.uber.org/fx"
import support "surfin/pkg/batch/core/config/support"
import logger "surfin/pkg/batch/support/util/logger"

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
