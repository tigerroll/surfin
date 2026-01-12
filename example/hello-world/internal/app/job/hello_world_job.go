package job

import (
	"context"

	config "surfin/pkg/batch/core/config"
	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	metrics "surfin/pkg/batch/core/metrics"
	logger "surfin/pkg/batch/support/util/logger"
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
