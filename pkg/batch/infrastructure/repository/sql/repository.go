package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	repository "github.com/tigerroll/surfin/pkg/batch/core/domain/repository"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

// GORMJobRepository implements the repository.JobRepository interface using GORM.
type GORMJobRepository struct {
	dbResolver adaptor.DBConnectionResolver // FIX: Use DBConnectionResolver.
	TxManager tx.TransactionManager // FIX: Export TxManager.
	dbName    string // The database connection name used by the JobRepository (e.g., "metadata").
}

// NewGORMJobRepository creates a new instance of GORMJobRepository.
func NewGORMJobRepository(
	dbResolver adaptor.DBConnectionResolver, // FIX: Use DBConnectionResolver.
	txManager tx.TransactionManager,
	dbName string, // ADD: dbName parameter.
) repository.JobRepository {
	return &GORMJobRepository{
		dbResolver: dbResolver,
		TxManager: txManager,
		dbName:    dbName,
	}
}

// isTableNotExistError checks if the given error is a "table does not exist" error from the database.
// This typically occurs when the JobRepository is accessed before migrations have been run.
func isTableNotExistError(err error) bool {
	if err == nil { return false }
	errMsg := err.Error()
	// PostgreSQL: ERROR: relation "..." does not exist (SQLSTATE 42P01).
	// MySQL/SQLite: no such table: ....
	return (strings.Contains(errMsg, "relation \"") && strings.Contains(errMsg, "\" does not exist")) ||
		strings.Contains(errMsg, "no such table:")
}

// getDBConnection is a helper function to get the DBConnection used by JobRepository.
// This is used for operations outside of a transaction (ExecuteQuery, Count, Pluck).
func (r *GORMJobRepository) getDBConnection(ctx context.Context) (adaptor.DBConnection, error) {
	// Use DBConnectionResolver to always get the latest DBConnection.
	conn, err := r.dbResolver.ResolveDBConnection(ctx, r.dbName)
	if err != nil {
		return nil, exception.NewBatchError("GORMJobRepository", fmt.Sprintf("Failed to resolve DB connection '%s'", r.dbName), err, false, false)
	}
	return conn, nil
}

// getTxExecutor checks if a Tx exists in the context.
// If it exists, it returns the Tx (TxExecutor); otherwise, it returns the DBConnection (which implements TxExecutor).
// This is used for operations within a transaction (ExecuteUpdate, ExecuteUpsert).
func (r *GORMJobRepository) getTxExecutor(ctx context.Context) (tx.TxExecutor, error) {
	// Get Tx from context.
	if t, ok := ctx.Value("tx").(tx.Tx); ok {
		return t, nil // If a transaction exists in the context, use it.
	}
	// If no transaction is found in the context, use the direct DBConnection.
	return r.getDBConnection(ctx)
}

/*
// getDBContext retrieves a GORM DB instance from a DBConnection and sets the Context.
// NOTE: This method should only be used when advanced GORM features (e.g., Preload) cannot be abstracted by DBConnection.
// To complete T_GORM_2, its use is restricted to SaveCheckpointData.
func (r *GORMJobRepository) getDBContext(ctx context.Context) (*gorm.DB, error) {
	gormDB, err := database.GetGormDBFromConnection(r.dbConn)
	if err != nil {
		return nil, exception.NewBatchError("GORMJobRepository", "Failed to get GORM DB connection", err, false, false)
	}
	return gormDB.WithContext(ctx), nil
}
*/

// --- JobInstance implementation ---

func (r *GORMJobRepository) SaveJobInstance(ctx context.Context, instance *model.JobInstance) error {
	const op = "GORMJobRepository.SaveJobInstance"
	entity := fromDomainJobInstance(instance)
	
	executor, err := r.getTxExecutor(ctx) // FIX: Use getTxExecutor.
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for INSERT
	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", entity.TableName(), nil)

	if err != nil {
		if isTableNotExistError(err) {
			// Ignore if table does not exist (e.g., before migrations are run).
			return nil
		}
		return exception.NewBatchError(op, fmt.Sprintf("failed to save JobInstance (ID: %s)", instance.ID), err, true, false)
	}
	return nil
}

func (r *GORMJobRepository) UpdateJobInstance(ctx context.Context, instance *model.JobInstance) error {
	const op = "GORMJobRepository.UpdateJobInstance"

	originalVersion := instance.Version
	instance.Version++
	entity := fromDomainJobInstance(instance)
	
	tableName := entity.TableName()
	executor, err := r.getTxExecutor(ctx) // FIX: Use getTxExecutor.
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for UPDATE and optimistic locking
	// The ID condition is automatically added because db.Model(entity) is used within GORM's ExecuteUpdate.
	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion}, // Remove ID condition
	)

	if err != nil {
		if isTableNotExistError(err) {
			// Ignore if table does not exist (e.g., before migrations are run).
			instance.Version = originalVersion // Rollback version
			return nil
		}
		instance.Version = originalVersion // Rollback version
		return exception.NewBatchError(op, fmt.Sprintf("failed to update JobInstance (ID: %s)", instance.ID), err, true, false)
	}
	if rowsAffected == 0 {
		instance.Version = originalVersion // Rollback version
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("JobInstance (ID: %s) with version %d not found for update", instance.ID, originalVersion), nil)
	}
	return nil
}

func (r *GORMJobRepository) FindJobInstanceByJobNameAndParameters(ctx context.Context, jobName string, params model.JobParameters) (*model.JobInstance, error) {
	const op = "GORMJobRepository.FindJobInstanceByJobNameAndParameters"
	hash, err := params.Hash()
	if err != nil {
		return nil, exception.NewBatchError(op, "failed to calculate JobParameters hash", err, false, false)
	}
	
	var entities []JobInstanceEntity
	
	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use ExecuteQuery to retrieve JobInstance with matching hash
	// NOTE: ExecuteQuery uses Find(), so it expects results to be returned as a slice.
	err = conn.ExecuteQuery(ctx, &entities, map[string]interface{}{"job_name": jobName, "parameters_hash": hash})
	
	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
			return nil, repository.ErrJobInstanceNotFound
		}
		// ExecuteQuery (Find) does not return ErrRecordNotFound, so only check for other DB errors here.
		return nil, exception.NewBatchError(op, "failed to find JobInstance", err, true, false)
	}
	
	if len(entities) == 0 {
		return nil, repository.ErrJobInstanceNotFound
	}

	// Check each retrieved instance to find the one with exact parameter match.
	for _, entity := range entities {
		domainInstance := toDomainJobInstance(&entity)
		if domainInstance.Parameters.Equal(params) {
			return domainInstance, nil
		}
		logger.Warnf("%s: JobInstance (ID: %s) hash matched but parameters mismatched. Possible hash collision.", op, domainInstance.ID)
	}

	return nil, repository.ErrJobInstanceNotFound // No instance found with matching parameters.
}

func (r *GORMJobRepository) FindJobInstanceByID(ctx context.Context, id string) (*model.JobInstance, error) {
	const op = "GORMJobRepository.FindJobInstanceByID"
	var entity JobInstanceEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use ExecuteQueryAdvanced to search by ID (with Limit 1)
	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": id}, "", 1)
	
	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
			return nil, repository.ErrJobInstanceNotFound
		}
		// ExecuteQuery (Find) does not return ErrRecordNotFound, so only catch other DB errors here.
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobInstance by ID: %s", id), err, true, false)
	}
	
	// If no record found
	if entity.ID == "" {
		return nil, repository.ErrJobInstanceNotFound
	}

	return toDomainJobInstance(&entity), nil
}

// FindJobInstancesByJobNameAndPartialParameters implements repository.JobInstance.
func (r *GORMJobRepository) FindJobInstancesByJobNameAndPartialParameters(ctx context.Context, jobName string, partialParams model.JobParameters) ([]*model.JobInstance, error) {
	const op = "GORMJobRepository.FindJobInstancesByJobNameAndPartialParameters"
	var entities []JobInstanceEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Filter by JobName
	query := map[string]interface{}{"job_name": jobName}

	// 2. Use ExecuteQueryAdvanced to search
	// NOTE: Exact parameter matching is done in memory, so only search by JobName here.
	err = conn.ExecuteQueryAdvanced(ctx, &entities, query, "create_time desc", 0)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, return empty slice.
			return []*model.JobInstance{}, nil
		}
		return nil, exception.NewBatchError(op, "failed to find JobInstances by job name", err, true, false)
	}

	if len(entities) == 0 {
		return []*model.JobInstance{}, nil
	}

	// 3. Check for partial parameter match
	var matchingInstances []*model.JobInstance
	for _, entity := range entities {
		domainInstance := toDomainJobInstance(&entity)
		if domainInstance.Parameters.Contains(partialParams) {
			matchingInstances = append(matchingInstances, domainInstance)
		}
	}

	return matchingInstances, nil
}

// GetJobInstanceCount implements repository.JobInstance.
func (r *GORMJobRepository) GetJobInstanceCount(ctx context.Context, jobName string) (int, error) {
	const op = "GORMJobRepository.GetJobInstanceCount"
	
	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return 0, err
	}

	count, err := conn.Count(ctx, &JobInstanceEntity{}, map[string]interface{}{"job_name": jobName})
	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, return 0.
			return 0, nil
		}
		return 0, exception.NewBatchError(op, "failed to count JobInstances", err, true, false)
	}
	return int(count), nil
}

// GetJobNames implements repository.JobInstance.
func (r *GORMJobRepository) GetJobNames(ctx context.Context) ([]string, error) {
	const op = "GORMJobRepository.GetJobNames"
	var jobNames []string
	
	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use Pluck to get a list of JobNames
	err = conn.Pluck(ctx, &JobInstanceEntity{}, "job_name", &jobNames, nil)
	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, return empty slice.
			return []string{}, nil
		}
		return nil, exception.NewBatchError(op, "failed to pluck job names", err, true, false)
	}
	return jobNames, nil
}

// --- JobExecution implementation ---

func (r *GORMJobRepository) SaveJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	const op = "GORMJobRepository.SaveJobExecution"
	entity := fromDomainJobExecution(jobExecution)
	
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for INSERT
	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", entity.TableName(), nil)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, ignore.
			return nil
		}
		return exception.NewBatchError(op, fmt.Sprintf("failed to save JobExecution (ID: %s)", jobExecution.ID), err, true, false)
	}
	return nil
}

func (r *GORMJobRepository) UpdateJobExecution(ctx context.Context, jobExecution *model.JobExecution) error {
	const op = "GORMJobRepository.UpdateJobExecution"

	originalVersion := jobExecution.Version
	jobExecution.Version++
	jobExecution.LastUpdated = time.Now()
	entity := fromDomainJobExecution(jobExecution)
	
	tableName := entity.TableName()
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for UPDATE and optimistic locking
	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion},
	)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, ignore.
			jobExecution.Version = originalVersion
			return nil
		}
		jobExecution.Version = originalVersion
		return exception.NewBatchError(op, fmt.Sprintf("failed to update JobExecution (ID: %s)", jobExecution.ID), err, true, false)
	}
	if rowsAffected == 0 {
		jobExecution.Version = originalVersion
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("JobExecution (ID: %s) with version %d not found for update", jobExecution.ID, originalVersion), nil)
	}
	return nil
}

func (r *GORMJobRepository) FindJobExecutionByID(ctx context.Context, executionID string) (*model.JobExecution, error) {
	const op = "GORMJobRepository.FindJobExecutionByID"
	var entity JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Load JobExecution
	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": executionID}, "", 1)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
			return nil, repository.ErrJobExecutionNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobExecution by ID: %s", executionID), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrJobExecutionNotFound
	}

	domainExecution := toDomainJobExecution(&entity)

	// 2. Load associated StepExecutions
	stepExecutions, err := r.FindStepExecutionsByJobExecutionID(ctx, executionID)
	if err != nil {
		// Failure to load StepExecution does not mean failure to load JobExecution, but log it.
		logger.Errorf("%s: Failed to load StepExecutions for JobExecution (ID: %s): %v", op, executionID, err)
		// Ignore the error and return the partially loaded JobExecution.
	} else {
		domainExecution.StepExecutions = stepExecutions
	}

	return domainExecution, nil
}

// FindStepExecutionsByJobExecutionID retrieves all StepExecutions associated with a JobExecution.
func (r *GORMJobRepository) FindStepExecutionsByJobExecutionID(ctx context.Context, jobExecutionID string) ([]*model.StepExecution, error) {
	const op = "GORMJobRepository.FindStepExecutionsByJobExecutionID"
	var entities []StepExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use ExecuteQueryAdvanced to search (sorted by start_time)
	err = conn.ExecuteQueryAdvanced(ctx, &entities, map[string]interface{}{"job_execution_id": jobExecutionID}, "start_time asc", 0)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, return empty slice.
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

func (r *GORMJobRepository) FindLatestRestartableJobExecution(ctx context.Context, jobInstanceID string) (*model.JobExecution, error) {
	const op = "GORMJobRepository.FindLatestRestartableJobExecution"
	var entity JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Load JobExecution
	// Filter by JobInstanceID and get the latest one (descending ID)
	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"job_instance_id": jobInstanceID}, "create_time desc", 1)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
			return nil, repository.ErrJobExecutionNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find latest JobExecution for JobInstance ID: %s", jobInstanceID), err, true, false)
	}

	if entity.ID == "" {
		return nil, repository.ErrJobExecutionNotFound
	}

	domainExecution := toDomainJobExecution(&entity)

	// 2. Load associated StepExecutions
	stepExecutions, err := r.FindStepExecutionsByJobExecutionID(ctx, domainExecution.ID)
	if err != nil {
		logger.Errorf("%s: Failed to load StepExecutions for JobExecution (ID: %s): %v", op, domainExecution.ID, err)
	} else {
		domainExecution.StepExecutions = stepExecutions
	}

	// 3. Check if restartable
	// Status must not be COMPLETED, ABANDONED, STOPPING_FAILED, and must be a terminal state.
	// In Spring Batch logic, only FAILED or STOPPED are restartable.
	if domainExecution.Status == model.BatchStatusFailed || domainExecution.Status == model.BatchStatusStopped {
		return domainExecution, nil
	}
	
	// If running or completed, return nil.
	return nil, repository.ErrJobExecutionNotFound
}

func (r *GORMJobRepository) FindJobExecutionsByJobInstance(ctx context.Context, jobInstance *model.JobInstance) ([]*model.JobExecution, error) {
	const op = "GORMJobRepository.FindJobExecutionsByJobInstance"
	var entities []JobExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by JobInstanceID and retrieve in descending order of creation time.
	err = conn.ExecuteQueryAdvanced(ctx, &entities, map[string]interface{}{"job_instance_id": jobInstance.ID}, "create_time desc", 0)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, return empty slice.
			return []*model.JobExecution{}, nil
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find JobExecutions for JobInstance ID: %s", jobInstance.ID), err, true, false)
	}

	domainExecutions := make([]*model.JobExecution, len(entities))
	for i, entity := range entities {
		domainExecutions[i] = toDomainJobExecution(&entity)
	}

	// StepExecution loading is not done here (use FindJobExecutionByID if needed).
	return domainExecutions, nil
}

// --- StepExecution implementation ---

func (r *GORMJobRepository) SaveStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	const op = "GORMJobRepository.SaveStepExecution"
	entity := fromDomainStepExecution(stepExecution)
	
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for INSERT
	_, err = executor.ExecuteUpdate(ctx, entity, "CREATE", entity.TableName(), nil)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, ignore.
			return nil
		}
		return exception.NewBatchError(op, fmt.Sprintf("failed to save StepExecution (ID: %s)", stepExecution.ID), err, true, false)
	}
	return nil
}

func (r *GORMJobRepository) UpdateStepExecution(ctx context.Context, stepExecution *model.StepExecution) error {
	const op = "GORMJobRepository.UpdateStepExecution"

	originalVersion := stepExecution.Version
	stepExecution.Version++
	stepExecution.LastUpdated = time.Now()
	entity := fromDomainStepExecution(stepExecution)
	
	tableName := entity.TableName()
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	// Use ExecuteUpdate for UPDATE and optimistic locking
	rowsAffected, err := executor.ExecuteUpdate(
		ctx,
		entity,
		"UPDATE",
		tableName,
		map[string]interface{}{"version": originalVersion},
	)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, ignore.
			stepExecution.Version = originalVersion
			return nil
		}
		stepExecution.Version = originalVersion
		return exception.NewBatchError(op, fmt.Sprintf("failed to update StepExecution (ID: %s)", stepExecution.ID), err, true, false)
	}
	if rowsAffected == 0 {
		stepExecution.Version = originalVersion
		return exception.NewOptimisticLockingFailureException("repository", fmt.Sprintf("StepExecution (ID: %s) with version %d not found for update", stepExecution.ID, originalVersion), nil)
	}
	return nil
}

func (r *GORMJobRepository) FindStepExecutionByID(ctx context.Context, executionID string) (*model.StepExecution, error) {
	const op = "GORMJobRepository.FindStepExecutionByID"
	var entity StepExecutionEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use ExecuteQueryAdvanced to search by ID (with Limit 1)
	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"id": executionID}, "", 1)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
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

func (r *GORMJobRepository) SaveCheckpointData(ctx context.Context, data *model.CheckpointData) error {
	const op = "GORMJobRepository.SaveCheckpointData"
	entity := fromDomainCheckpointData(data)
	
	executor, err := r.getTxExecutor(ctx)
	if err != nil {
		return err
	}

	// Use ExecuteUpsert to perform INSERT OR REPLACE / ON CONFLICT DO UPDATE
	// Conflict Columns: step_execution_id
	// Update Columns: execution_context, last_updated
	conflictCols := []string{"step_execution_id"}
	updateCols := []string{"execution_context", "last_updated"}

	// ExecuteUpsert is expected to be executed within a transaction.
	_, err = executor.ExecuteUpsert(ctx, entity, entity.TableName(), conflictCols, updateCols)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, ignore.
			return nil
		}
		return exception.NewBatchError(op, fmt.Sprintf("failed to save CheckpointData for StepExecution (ID: %s)", data.StepExecutionID), err, true, false)
	}
	return nil
}

func (r *GORMJobRepository) FindCheckpointData(ctx context.Context, stepExecutionID string) (*model.CheckpointData, error) {
	const op = "GORMJobRepository.FindCheckpointData"
	var entity CheckpointDataEntity

	conn, err := r.getDBConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Use ExecuteQueryAdvanced to search by ID (with Limit 1)
	err = conn.ExecuteQueryAdvanced(ctx, &entity, map[string]interface{}{"step_execution_id": stepExecutionID}, "", 1)

	if err != nil {
		if isTableNotExistError(err) { // If table does not exist, treat as not found.
			return nil, repository.ErrCheckpointDataNotFound
		}
		return nil, exception.NewBatchError(op, fmt.Sprintf("failed to find CheckpointData by StepExecution ID: %s", stepExecutionID), err, true, false)
	}

	if entity.StepExecutionID == "" {
		return nil, repository.ErrCheckpointDataNotFound
	}

	return toDomainCheckpointData(&entity), nil
}

// Close implements repository.JobRepository.
func (r *GORMJobRepository) Close() error {
	// DBConnection is managed by DBProvider, so it is not closed here.
	return nil
}

// Verify that GORMJobRepository implements all embedded interfaces of repository.JobRepository.
var _ repository.JobRepository = (*GORMJobRepository)(nil)

// JobRepositoryParams defines the dependencies required by NewJobRepository.
type JobRepositoryParams struct {
	fx.In
	DBResolver adaptor.DBConnectionResolver
	MetadataTxManager tx.TransactionManager `name:"metadata"`
	Cfg *config.Config
}

// NewJobRepository creates and returns a JobRepository instance.
// This function is intended to be used as an Fx provider.
func NewJobRepository(p JobRepositoryParams) repository.JobRepository {
	// Get the DB connection name used by the JobRepository (usually "metadata").
	dbName := p.Cfg.Surfin.Infrastructure.JobRepositoryDBRef
	if dbName == "" {
		dbName = "metadata"
	}
	
	return NewGORMJobRepository(p.DBResolver, p.MetadataTxManager, dbName)
}
