package runner

import (
	"context"

	port "surfin/pkg/batch/core/application/port"
	model "surfin/pkg/batch/core/domain/model"
	repository "surfin/pkg/batch/core/domain/repository"
	metrics "surfin/pkg/batch/core/metrics"
	exception "surfin/pkg/batch/support/util/exception"
	logger "surfin/pkg/batch/support/util/logger"
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
