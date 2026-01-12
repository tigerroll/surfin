package sql

import (
	"surfin/pkg/batch/core/domain/model"
)

// --- Mapper functions ---

func fromDomainJobInstance(ji *model.JobInstance) *JobInstanceEntity {
	if ji == nil {
		return nil
	}
	return &JobInstanceEntity{
		ID:             ji.ID,
		JobName:        ji.JobName,
		Parameters:     ji.Parameters,
		CreateTime:     ji.CreateTime,
		Version:        ji.Version,
		ParametersHash: ji.ParametersHash,
	}
}

func toDomainJobInstance(entity *JobInstanceEntity) *model.JobInstance {
	if entity == nil {
		return nil
	}
	return &model.JobInstance{
		ID:             entity.ID,
		JobName:        entity.JobName,
		Parameters:     entity.Parameters,
		CreateTime:     entity.CreateTime,
		Version:        entity.Version,
		ParametersHash: entity.ParametersHash,
	}
}

func fromDomainJobExecution(je *model.JobExecution) *JobExecutionEntity {
	if je == nil {
		return nil
	}
	entity := &JobExecutionEntity{
		ID:               je.ID,
		JobInstanceID:    je.JobInstanceID,
		JobName:          je.JobName,
		Parameters:       je.Parameters,
		StartTime:        je.StartTime,
		EndTime:          je.EndTime,
		Status:           je.Status,
		ExitStatus:       je.ExitStatus,
		ExitCode:         je.ExitCode,
		Failures:         je.Failures,
		Version:          je.Version,
		CreateTime:       je.CreateTime,
		LastUpdated:      je.LastUpdated,
		ExecutionContext: je.ExecutionContext,
		CurrentStepName:  je.CurrentStepName,
		RestartCount:     je.RestartCount,
	}
	// The StepExecutions field has been removed from the entity, so related processing is also removed.
	
	return entity
}

func toDomainJobExecution(entity *JobExecutionEntity) *model.JobExecution {
	if entity == nil {
		return nil
	}
	je := &model.JobExecution{
		ID:               entity.ID,
		JobInstanceID:    entity.JobInstanceID,
		JobName:          entity.JobName,
		Parameters:       entity.Parameters,
		StartTime:        entity.StartTime,
		EndTime:          entity.EndTime,
		Status:           entity.Status,
		ExitStatus:       entity.ExitStatus,
		ExitCode:         entity.ExitCode,
		Failures:         entity.Failures,
		Version:          entity.Version,
		CreateTime:       entity.CreateTime,
		LastUpdated:      entity.LastUpdated,
		ExecutionContext: entity.ExecutionContext,
		CurrentStepName:  entity.CurrentStepName,
		RestartCount:     entity.RestartCount,
		// CancelFunc is runtime-only and not persisted
	}
	// StepExecutions are manually loaded in the repository layer, so an empty slice is initialized here.
	je.StepExecutions = make([]*model.StepExecution, 0)
	
	return je
}

func fromDomainStepExecution(se *model.StepExecution) *StepExecutionEntity {
	if se == nil {
		return nil
	}
	entity := &StepExecutionEntity{
		ID:               se.ID,
		StepName:         se.StepName,
		JobExecutionID:   se.JobExecutionID,
		StartTime:        se.StartTime,
		EndTime:          se.EndTime,
		Status:           se.Status,
		ExitStatus:       se.ExitStatus,
		Failures:         se.Failures,
		ReadCount:        se.ReadCount,
		WriteCount:       se.WriteCount,
		CommitCount:      se.CommitCount,
		RollbackCount:    se.RollbackCount,
		FilterCount:      se.FilterCount,
		SkipReadCount:    se.SkipReadCount,
		SkipProcessCount: se.SkipProcessCount,
		SkipWriteCount:   se.SkipWriteCount,
		ExecutionContext: se.ExecutionContext,
		LastUpdated:      se.LastUpdated,
		Version:          se.Version,
	}
	// The JobExecution field has been removed from the entity, so related processing is also removed.
	
	return entity
}

func toDomainStepExecution(entity *StepExecutionEntity) *model.StepExecution {
	if entity == nil {
		return nil
	}
	return &model.StepExecution{
		ID:               entity.ID,
		StepName:         entity.StepName,
		JobExecutionID:   entity.JobExecutionID,
		StartTime:        entity.StartTime,
		EndTime:          entity.EndTime,
		Status:           entity.Status,
		ExitStatus:       entity.ExitStatus,
		Failures:         entity.Failures,
		ReadCount:        entity.ReadCount,
		WriteCount:       entity.WriteCount,
		CommitCount:      entity.CommitCount,
		RollbackCount:    entity.RollbackCount,
		FilterCount:      entity.FilterCount,
		SkipReadCount:    entity.SkipReadCount,
		SkipProcessCount: entity.SkipProcessCount,
		SkipWriteCount:   entity.SkipWriteCount,
		ExecutionContext: entity.ExecutionContext,
		LastUpdated:      entity.LastUpdated,
		Version:          entity.Version,
		// JobExecution is hydrated by the caller (e.g., toDomainJobExecution) to avoid cycles
	}
}

func fromDomainCheckpointData(cd *model.CheckpointData) *CheckpointDataEntity {
	if cd == nil {
		return nil
	}
	return &CheckpointDataEntity{
		StepExecutionID:  cd.StepExecutionID,
		ExecutionContext: cd.ExecutionContext,
		LastUpdated:      cd.LastUpdated,
	}
}

func toDomainCheckpointData(entity *CheckpointDataEntity) *model.CheckpointData {
	if entity == nil {
		return nil
	}
	return &model.CheckpointData{
		StepExecutionID:  entity.StepExecutionID,
		ExecutionContext: entity.ExecutionContext,
		LastUpdated:      entity.LastUpdated,
	}
}
