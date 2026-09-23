package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	coreAdapter "github.com/tigerroll/surfin/pkg/batch/core/adapter"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"go.uber.org/fx"
)

// SQLJobRepository implements the repository.JobRepository interface using SQL database operations.
type SQLJobRepository struct {
	// dbResolver is used to resolve database connections dynamically.
	dbResolver coreAdapter.ResourceConnectionResolver
	// TxManager is the transaction manager for the database.
	TxManager tx.TransactionManager
	// dbName is the name of the database connection used by this repository.
	dbName string
	// schema is the database schema name.
	schema string
}

// NewSQLJobRepository creates a new instance of SQLJobRepository.
//
// Parameters:
//
//	dbResolver: The database connection resolver.
//	txManager: The transaction manager for the database.
//	dbName: The name of the database connection to be used by this repository.
//	schema: The database schema name.
//
// Returns:
//
//	A new instance of repository.JobRepository.
func NewSQLJobRepository(
	dbResolver coreAdapter.ResourceConnectionResolver,
	txManager tx.TransactionManager,
	dbName string,
	schema string,
) repository.JobRepository {
	return &SQLJobRepository{
		dbResolver: dbResolver,
		TxManager:  txManager,
		dbName:     dbName,
		schema:     schema,
	}
}

// getFullTableName returns the table name prefixed with the schema if configured.
func (r *SQLJobRepository) getFullTableName(tableName string) string {
	if r.schema != "" {
		return fmt.Sprintf("%s.%s", r.schema, tableName)
	}
	return tableName
}

// getDBConnection retrieves the DBConnection used by the JobRepository.
// This is used for operations that do not require an active transaction (e.g., ExecuteQuery, Count, Pluck).
func (r *SQLJobRepository) getDBConnection(ctx context.Context) (database.DBConnection, error) {
	connAsResource, err := r.dbResolver.ResolveConnection(ctx, r.dbName)
	if err != nil {
		return nil, exception.NewBatchError("SQLJobRepository", fmt.Sprintf("Failed to resolve DB connection '%s'", r.dbName), err, false, false)
	}
	conn, ok := connAsResource.(database.DBConnection)
	if !ok {
		return nil, exception.NewBatchError("SQLJobRepository", fmt.Sprintf("Resolved connection '%s' is not a database.DBConnection", r.dbName), nil, false, false)
	}
	return conn, nil
}

// getTxExecutor returns a TxExecutor.
// If a transaction exists in the context, it returns the Tx; otherwise, it returns the DBConnection.
// This is used for operations within a transaction (e.g., ExecuteUpdate, ExecuteUpsert).
func (r *SQLJobRepository) getTxExecutor(ctx context.Context) (tx.TxExecutor, error) {
	if t, ok := tx.TxFromContext(ctx); ok {
		return t, nil
	}
	return r.getDBConnection(ctx)
}

// --- JobInstance implementation ---

// SaveJobInstance persists a new JobInstance.
func (r *SQLJobRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	const op = "SQLJobRepository.SaveJobInstance"
	entity := fromDomainJobInstance(instance)

	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", r.getFullTableName(entity.TableName()), nil)

	if err != nil {
		return exception.NewBatchError(op, fmt.Sprintf("failed to save JobInstance (ID: %s)", instance.ID), err, true, false)
	}
	return nil
}

// UpdateJobInstance updates the state of an existing JobInstance.
func (r *SQLJobRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	const op = "SQLJobRepository.UpdateJobInstance"

	originalVersion := instance.Version
	instance.Version++
	entity := fromDomainJobInstance(instance)

	tableName := r.getFullTableName(entity.TableName())
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion},
	)
	if err != nil {
		instance.Version = originalVersion
		return exception.NewBatchError(op, fmt.Sprintf("failed to update JobInstance (ID: %s)", instance.ID), err, true, false)
	}
	if rowsAffected == 0 {
		instance.Version = originalVersion
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("JobInstance (ID: %s) with version %d not found for update", instance.ID, originalVersion), nil)
	}
	return nil
}

// FindJobInstanceByJobNameAndParameters finds a JobInstance by job name and parameters.
func (r *SQLJobRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	const op = "SQLJobRepository.FindJobInstanceByJobNameAndParameters"
	hash, err := params.Hash()
	if err != nil {
		return nil, exception.NewBatchError(op, "failed to calculate JobParameters hash", err, false, false)
	}

	var entities []JobInstanceEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQuery(ctx, &entities, map[string]interface{}{"job_name": jobName, "parameters_hash": hash})

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrJobInstanceNotFound
		}
		return nil, exception.NewBatchError(op, "failed to find JobInstance", err, true, false)
	}

	if len(entities) == 0 {
		return nil, repository.ErrJobInstanceNotFound
	}

	for _, entity := range entities {
		domainInstance := toDomainJobInstance(&entity)
		if domainInstance.Parameters.Equal(params) {
			return domainInstance, nil
		}
		logger.Warnf("%s: JobInstance (ID: %s) hash matched but parameters mismatched. Possible hash collision.", op, domainInstance.ID)
	}

	return nil, repository.ErrJobInstanceNotFound
}

// FindJobInstanceByID finds a JobInstance by its ID.
func (r *SQLJobRepository) FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error) {
	const op = "SQLJobRepository.FindJobInstanceByID"
	var entity JobInstanceEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": id}, "", 1)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrJobInstanceNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobInstance by ID: %s", id), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrJobInstanceNotFound
	}

	return toDomainJobInstance(&entity), nil
}

// FindJobInstancesByJobNameAndPartialParameters finds JobInstances by job name and partial parameters.
func (r *SQLJobRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	const op = "SQLJobRepository.FindJobInstancesByJobNameAndPartialParameters"
	var entities []JobInstanceEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	query := map[string]interface{}{"job_name": jobName}

	err = conn.ExecuteQueryAdvanced(ctx, &entities, query, "create_time desc", 0)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return []*model.JobInstance{}, nil
		}
		return nil, exception.NewBatchError(op, "failed to find JobInstances by job name", err, true, false)
	}

	if len(entities) == 0 {
		return []*model.JobInstance{}, nil
	}

	var matchingInstances []*model.JobInstance
	for _, entity := range entities {
		domainInstance := toDomainJobInstance(&entity)
		if domainInstance.Parameters.Contains(partialParams) {
			matchingInstances = append(matchingInstances, domainInstance)
		}
	}

	return matchingInstances, nil
}

// GetJobInstanceCount returns the count of JobInstances for a given job name.
func (r *SQLJobRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	const op = "SQLJobRepository.GetJobInstanceCount"

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return 0, err
	}
	count, err := conn.Count(ctx, &JobInstanceEntity{}, map[string]interface{}{"job_name": jobName})
	if err != nil {
		if conn.IsTableNotExistError(err) {
			return 0, nil
		}
		return 0, exception.NewBatchError(op, "failed to count JobInstances", err, true, false)
	}
	return int(count), nil
}

// GetJobNames returns a list of all job names.
func (r *SQLJobRepository) GetJobNames(ctx context.Context) ([]string, error) {
	const op = "SQLJobRepository.GetJobNames"
	var jobNames []string

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.Pluck(ctx, &JobInstanceEntity{}, "job_name", &jobNames, nil)
	if err != nil {
		if conn.IsTableNotExistError(err) {
			return []string{}, nil
		}
		return nil, exception.NewBatchError(op, "failed to pluck job names", err, true, false)
	}
	return jobNames, nil
}

// --- JobExecution implementation ---

// SaveJobExecution persists a new JobExecution.
func (r *SQLJobRepository) SaveJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	const op = "SQLJobRepository.SaveJobExecution"
	entity := fromDomainJobExecution(jobExecution)

	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", r.getFullTableName(entity.TableName()), nil)

	if err != nil {
		return exception.NewBatchError(op, fmt.Sprintf("failed to save JobExecution (ID: %s)", jobExecution.ID), err, true, false)
	}
	return nil
}

// UpdateJobExecution updates the state of an existing JobExecution.
func (r *SQLJobRepository) UpdateJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	const op = "SQLJobRepository.UpdateJobExecution"

	originalVersion := jobExecution.Version
	jobExecution.Version++
	jobExecution.LastUpdated = time.Now()
	entity := fromDomainJobExecution(jobExecution)

	tableName := r.getFullTableName(entity.TableName())
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion},
	)
	if err != nil {
		jobExecution.Version = originalVersion
		return exception.NewBatchError(op, fmt.Sprintf("failed to update JobExecution (ID: %s)", jobExecution.ID), err, true, false)
	}
	if rowsAffected == 0 {
		jobExecution.Version = originalVersion
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("JobExecution (ID: %s) with version %d not found for update", jobExecution.ID, originalVersion), nil)
	}
	return nil
}

// FindJobExecutionByID finds a JobExecution by its ID.
func (r *SQLJobRepository) FindJobExecutionByID(ctx context.Context, executionID string) (*model.JobExecution, error) {
	const op = "SQLJobRepository.FindJobExecutionByID"
	var entity JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": executionID}, "", 1)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrJobExecutionNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobExecution by ID: %s", executionID), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrJobExecutionNotFound
	}

	domainExecution := toDomainJobExecution(&entity)

	stepExecutions, err := r.FindStepExecutionsByJobExecutionID(ctx, executionID)
	if err != nil {
		logger.Errorf("%s: Failed to load StepExecutions for JobExecution (ID: %s): %v", op, executionID, err)
	} else {
		domainExecution.StepExecutions = stepExecutions
	}

	return domainExecution, nil
}

// FindStepExecutionsByJobExecutionID retrieves all StepExecutions associated with a JobExecution.
func (r *SQLJobRepository) FindStepExecutionsByJobExecutionID(ctx context.Context, jobExecutionID string) ([]*model.StepExecution, error) {
	const op = "SQLJobRepository.FindStepExecutionsByJobExecutionID"
	var entities []StepExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entities, map[string]interface{}{"job_execution_id": jobExecutionID}, "start_time asc", 0)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return []*model.StepExecution{}, nil
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find StepExecutions by JobExecution ID: %s", jobExecutionID), err, true, false)
	}

	domainExecutions := make([]*model.StepExecution, len(entities))
	for i, entity := range entities {
		domainExecutions[i] = toDomainStepExecution(&entity)
	}

	return domainExecutions, nil
}

// FindLatestRestartableJobExecution finds the latest restartable JobExecution for a given JobInstance.
func (r *SQLJobRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	const op = "SQLJobRepository.FindLatestRestartableJobExecution"
	var entity JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"job_instance_id": jobInstanceID}, "create_time desc", 1)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrJobExecutionNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find latest JobExecution for JobInstance ID: %s", jobInstanceID), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrJobExecutionNotFound
	}

	domainExecution := toDomainJobExecution(&entity)

	stepExecutions, err := r.FindStepExecutionsByJobExecutionID(ctx, domainExecution.ID)
	if err != nil {
		logger.Errorf("%s: Failed to load StepExecutions for JobExecution (ID: %s): %v", op, domainExecution.ID, err)
	} else {
		domainExecution.StepExecutions = stepExecutions
	}

	if domainExecution.Status == model.BatchStatusFailed || domainExecution.Status == model.BatchStatusStopped {
		return domainExecution, nil
	}

	return nil, repository.ErrJobExecutionNotFound
}

// FindJobExecutionsByJobInstance finds all JobExecutions for a given JobInstance.
func (r *SQLJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, jobInstance *model.JobInstance) ([]*model.JobExecution, error) {
	const op = "SQLJobRepository.FindJobExecutionsByJobInstance"
	var entities []JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entities, map[string]interface{}{"job_instance_id": jobInstance.ID}, "create_time desc", 0)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return []*model.JobExecution{}, nil
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobExecutions for JobInstance ID: %s", jobInstance.ID), err, true, false)
	}

	if len(entities) == 0 {
		return []*model.JobExecution{}, nil
	}

	domainExecutions := make([]*model.JobExecution, len(entities))
	for i, entity := range entities {
		domainExecutions[i] = toDomainJobExecution(&entity)
	}

	return domainExecutions, nil
}

// --- StepExecution implementation ---

// SaveStepExecution persists a new StepExecution.
func (r *SQLJobRepository) SaveStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	const op = "SQLJobRepository.SaveStepExecution"
	entity := fromDomainStepExecution(stepExecution)

	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", r.getFullTableName(entity.TableName()), nil)

	if err != nil {
		return exception.NewBatchError(op, fmt.Sprintf("failed to save StepExecution (ID: %s)", stepExecution.ID), err, true, false)
	}
	return nil
}

// UpdateStepExecution updates the state of an existing StepExecution.
func (r *SQLJobRepository) UpdateStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	const op = "SQLJobRepository.UpdateStepExecution"

	originalVersion := stepExecution.Version
	stepExecution.Version++
	stepExecution.LastUpdated = time.Now()
	entity := fromDomainStepExecution(stepExecution)

	tableName := r.getFullTableName(entity.TableName())
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion},
	)
	if err != nil {
		stepExecution.Version = originalVersion
		return exception.NewBatchError(op, fmt.Sprintf("failed to update StepExecution (ID: %s)", stepExecution.ID), err, true, false)
	}
	if rowsAffected == 0 {
		stepExecution.Version = originalVersion
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("StepExecution (ID: %s) with version %d not found for update", stepExecution.ID, originalVersion), nil)
	}
	return nil
}

// FindStepExecutionByID finds a StepExecution by its ID.
func (r *SQLJobRepository) FindStepExecutionByID(ctx context.Context, executionID string) (*model.StepExecution, error) {
	const op = "SQLJobRepository.FindStepExecutionByID"
	var entity StepExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": executionID}, "", 1)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrStepExecutionNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find StepExecution by ID: %s", executionID), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrStepExecutionNotFound
	}

	return toDomainStepExecution(&entity), nil
}

// --- CheckpointData implementation ---

// SaveCheckpointData persists CheckpointData.
func (r *SQLJobRepository) SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error {
	const op = "SQLJobRepository.SaveCheckpointData"

	if data.LastUpdated.IsZero() {
		data.LastUpdated = time.Now()
	}
	entity := fromDomainCheckpointData(data)

	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	conflictCols := []string{"step_execution_id"}
	updateCols := []string{"execution_context", "last_updated"}

	_, err = executor.ExecuteUpsert(ctx, entity, r.getFullTableName(entity.TableName()), conflictCols, updateCols)

	if err != nil {
		return exception.NewBatchError(op, fmt.Sprintf("failed to save CheckpointData for StepExecution (ID: %s)", data.StepExecutionID), err, true, false)
	}
	return nil
}

// FindCheckpointData finds CheckpointData by StepExecution ID.
func (r *SQLJobRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	const op = "SQLJobRepository.FindCheckpointData"
	var entity CheckpointDataEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"step_execution_id": stepExecutionID}, "", 1)

	if err != nil {
		if conn.IsTableNotExistError(err) {
			return nil, repository.ErrCheckpointDataNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find CheckpointData by StepExecution ID: %s", stepExecutionID), err, true, false)
	}

	if entity.StepExecutionID == "" {
		return nil, repository.ErrCheckpointDataNotFound
	}

	return toDomainCheckpointData(&entity), nil
}

// Close releases resources used by the repository.
func (r *SQLJobRepository) Close() error {
	return nil
}

// Verify that SQLJobRepository implements all embedded interfaces of repository.JobRepository.
var _ repository.JobRepository = (*SQLJobRepository)(nil)

// JobRepositoryParams defines the dependencies required to create a NewJobRepository.
type JobRepositoryParams struct {
	fx.In
	DBResolver coreAdapter.ResourceConnectionResolver
	// MetadataTxManager is the transaction manager for the metadata database.
	MetadataTxManager tx.TransactionManager `name:"metadata"`
	// Cfg is the application configuration.
	Cfg *config.Config
}

// NewJobRepository creates and returns a JobRepository instance.
// This function is intended to be used as an Fx provider.
func NewJobRepository(p JobRepositoryParams) repository.JobRepository {
	dbName := p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef
	if dbName == "" {
		dbName = "metadata"
	}

	var schema string
	if dbConfigs, ok := p.Cfg.Surfin.AdapterConfigs["database"].(map[string]interface{}); ok {
		if dbConfig, ok := dbConfigs[dbName].(map[string]interface{}); ok {
			if s, ok := dbConfig["schema"].(string); ok {
				schema = s
			}
		}
	}

	return NewSQLJobRepository(p.DBResolver, p.MetadataTxManager, dbName, schema)
}
