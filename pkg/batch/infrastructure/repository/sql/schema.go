package sql

import (
	model "surfin/pkg/batch/core/domain/model"
	"time"
)

// JobInstanceEntity is a schema model used for persistence.
type JobInstanceEntity struct {
	ID             string              
	JobName        string              
	Parameters     model.JobParameters 
	CreateTime     time.Time           
	Version        int                 
	ParametersHash string              
}

func (JobInstanceEntity) TableName() string {
	return "batch_job_instance"
}

// JobExecutionEntity is a schema model used for persistence.
type JobExecutionEntity struct {
	ID               string                 
	JobInstanceID    string                 
	JobName          string                 
	Parameters       model.JobParameters    
	StartTime        time.Time              
	EndTime          *time.Time             
	Status           model.JobStatus        
	ExitStatus       model.ExitStatus       
	ExitCode         int                    
	Failures         model.FailureList      
	Version          int                    
	CreateTime       time.Time              
	LastUpdated      time.Time              
	ExecutionContext model.ExecutionContext 
	CurrentStepName  string                 
	RestartCount     int
	// StepExecutions []*StepExecutionEntity // Removed to avoid GORM schema parsing errors.
}

func (JobExecutionEntity) TableName() string {
	return "batch_job_execution"
}

// StepExecutionEntity is a schema model used for persistence.
type StepExecutionEntity struct {
	ID               string                 
	StepName         string                 
	JobExecutionID   string                 
	StartTime        time.Time              
	EndTime          *time.Time             
	Status           model.JobStatus        
	ExitStatus       model.ExitStatus       
	Failures         model.FailureList      
	ReadCount        int                    
	WriteCount       int                    
	CommitCount      int                    
	RollbackCount    int                    
	FilterCount      int                    
	SkipReadCount    int                    
	SkipProcessCount int                    
	SkipWriteCount   int                    
	ExecutionContext model.ExecutionContext 
	LastUpdated      time.Time
	Version          int
	// JobExecution *JobExecutionEntity // Removed to avoid GORM schema parsing errors.
}

func (StepExecutionEntity) TableName() string {
	return "batch_step_execution"
}

// CheckpointDataEntity is a schema model used for persistence.
type CheckpointDataEntity struct {
	StepExecutionID  string                 
	ExecutionContext model.ExecutionContext 
	LastUpdated      time.Time              
}

func (CheckpointDataEntity) TableName() string {
	return "batch_checkpoint_data"
}
