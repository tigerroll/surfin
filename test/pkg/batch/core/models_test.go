package core_test

import (
	"errors"
	"testing"
	"time"

	model "surfin/pkg/batch/core/domain/model"
	"surfin/pkg/batch/support/util/exception"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a basic JobExecution
func newTestJobExecution(status model.JobStatus) *model.JobExecution {
	je := model.NewJobExecution(uuid.New().String(), "testJob", model.NewJobParameters())
	je.Status = status
	return je
}

// Helper function to create a basic StepExecution
func newTestStepExecution(jobExec *model.JobExecution, status model.JobStatus) *model.StepExecution {
	se := model.NewStepExecution(uuid.New().String(), jobExec, "testStep")
	se.Status = status
	return se
}

func TestJobExecution_TransitionTo(t *testing.T) {
	// Test valid transitions
	je := newTestJobExecution(model.BatchStatusStarting)
	assert.NoError(t, je.TransitionTo(model.BatchStatusStarted))
	assert.Equal(t, model.BatchStatusStarted, je.Status)

	// STARTING -> FAILED (e.g., Job execution failed immediately during setup)
	je = newTestJobExecution(model.BatchStatusStarting)
	assert.NoError(t, je.TransitionTo(model.BatchStatusFailed))
	assert.Equal(t, model.BatchStatusFailed, je.Status)

	je = newTestJobExecution(model.BatchStatusStarted)
	assert.NoError(t, je.TransitionTo(model.BatchStatusStopping))
	assert.Equal(t, model.BatchStatusStopping, je.Status)

	// STOPPING -> STOPPED (Normal stop completion)
	je = newTestJobExecution(model.BatchStatusStopping)
	assert.NoError(t, je.TransitionTo(model.BatchStatusStopped))
	assert.Equal(t, model.BatchStatusStopped, je.Status)

	// RESTARTING -> STARTED (Successful restart)
	je = newTestJobExecution(model.BatchStatusRestarting)
	assert.NoError(t, je.TransitionTo(model.BatchStatusStarted))
	assert.Equal(t, model.BatchStatusStarted, je.Status)

	// RESTARTING -> FAILED (Restart failed immediately)
	je = newTestJobExecution(model.BatchStatusRestarting)
	assert.NoError(t, je.TransitionTo(model.BatchStatusFailed))
	assert.Equal(t, model.BatchStatusFailed, je.Status)

	// --- Restart Transitions from Finished States ---

	// FAILED -> ABANDONED (Cleanup for restart)
	je = newTestJobExecution(model.BatchStatusFailed)
	assert.NoError(t, je.TransitionTo(model.BatchStatusAbandoned))
	assert.Equal(t, model.BatchStatusAbandoned, je.Status)

	// FAILED -> RESTARTING (Direct restart preparation - new transition)
	je = newTestJobExecution(model.BatchStatusFailed)
	assert.NoError(t, je.TransitionTo(model.BatchStatusRestarting))
	assert.Equal(t, model.BatchStatusRestarting, je.Status)

	// STOPPED -> RESTARTING (Restart preparation)
	je = newTestJobExecution(model.BatchStatusStopped)
	assert.NoError(t, je.TransitionTo(model.BatchStatusRestarting))
	assert.Equal(t, model.BatchStatusRestarting, je.Status)

	// --- Invalid Transitions ---

	// STOPPING -> RESTARTING (Invalid: Should transition to STOPPED/FAILED first)
	je = newTestJobExecution(model.BatchStatusStopping)
	err := je.TransitionTo(model.BatchStatusRestarting)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")

	// FAILED -> STARTED (Invalid)
	je = newTestJobExecution(model.BatchStatusFailed)
	err = je.TransitionTo(model.BatchStatusStarted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")

	// COMPLETED -> STARTED (Invalid)
	je = newTestJobExecution(model.BatchStatusCompleted)
	err = je.TransitionTo(model.BatchStatusStarted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")

	// ABANDONED -> STARTED (Invalid)
	je = newTestJobExecution(model.BatchStatusAbandoned)
	err = je.TransitionTo(model.BatchStatusStarted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")


	// FAILED -> FAILED (Self-transition is invalid for FAILED)
	je = newTestJobExecution(model.BatchStatusFailed)
	err = je.TransitionTo(model.BatchStatusFailed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")
}

func TestJobStatus_ToExitStatus(t *testing.T) {
	tests := map[model.JobStatus]model.ExitStatus{
		model.BatchStatusCompleted: model.ExitStatusCompleted,
		model.BatchStatusFailed:    model.ExitStatusFailed,
		model.BatchStatusStopped:   model.ExitStatusStopped,
		model.BatchStatusAbandoned: model.ExitStatusAbandoned,
		model.BatchStatusStarted:   model.ExitStatusUnknown, // Non-finished status maps to UNKNOWN
		model.BatchStatusStarting:  model.ExitStatusUnknown,
	}
	for jobStatus, expectedExitStatus := range tests {
		assert.Equal(t, expectedExitStatus, jobStatus.ToExitStatus(), "JobStatus %s should map to ExitStatus %s", jobStatus, expectedExitStatus)
	}
}

func TestJobExecution_MarkStatusHelpers(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusStarting)
	initialLastUpdated := je.LastUpdated

	// MarkAsStarted
	time.Sleep(1 * time.Millisecond) // Ensure time advances
	je.MarkAsStarted()
	assert.Equal(t, model.BatchStatusStarted, je.Status)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
	initialLastUpdated = je.LastUpdated

	// MarkAsCompleted
	time.Sleep(1 * time.Millisecond)
	je.MarkAsCompleted()
	assert.Equal(t, model.BatchStatusCompleted, je.Status)
	assert.Equal(t, model.ExitStatusCompleted, je.ExitStatus)
	assert.NotNil(t, je.EndTime)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
	initialLastUpdated = je.LastUpdated

	// MarkAsFailed
	je = newTestJobExecution(model.BatchStatusStarting)
	time.Sleep(1 * time.Millisecond)
	testErr := errors.New("test failure")
	je.MarkAsFailed(testErr)
	assert.Equal(t, model.BatchStatusFailed, je.Status)
	assert.Equal(t, model.ExitStatusFailed, je.ExitStatus)
	assert.NotNil(t, je.EndTime)
	assert.Len(t, je.Failures, 1)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
	initialLastUpdated = je.LastUpdated

	// MarkAsStopped
	je = newTestJobExecution(model.BatchStatusStarting)
	time.Sleep(1 * time.Millisecond)
	je.MarkAsStopped()
	assert.Equal(t, model.BatchStatusStopped, je.Status)
	assert.Equal(t, model.ExitStatusStopped, je.ExitStatus)
	assert.NotNil(t, je.EndTime)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
	initialLastUpdated = je.LastUpdated

	// MarkAsAbandoned
	je = newTestJobExecution(model.BatchStatusStarting)
	time.Sleep(1 * time.Millisecond)
	je.MarkAsAbandoned()
	assert.Equal(t, model.BatchStatusAbandoned, je.Status)
	assert.Equal(t, model.ExitStatusAbandoned, je.ExitStatus)
	assert.NotNil(t, je.EndTime)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
}

func TestJobExecution_AddFailureException_Deduplication(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusStarted)
	err1 := errors.New("database connection failed")
	err2 := errors.New("database connection failed")
	err3 := errors.New("another error")

	je.AddFailureException(err1)
	assert.Len(t, je.Failures, 1)

	je.AddFailureException(err2) // Duplicate
	assert.Len(t, je.Failures, 1)

	je.AddFailureException(err3) // New error
	assert.Len(t, je.Failures, 2)
	assert.Equal(t, "database connection failed", je.Failures[0])
	assert.Equal(t, "another error", je.Failures[1])
}

func TestJobExecution_MarkAsFailed_Deduplication(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusStarted)

	// 1. 最初の失敗を追加
	err1 := errors.New("critical system failure")
	je.MarkAsFailed(err1)
	assert.Equal(t, model.BatchStatusFailed, je.Status)
	assert.Len(t, je.Failures, 1)

	// Status を STARTED に戻して、別のステップが失敗した状況をシミュレート
	// MarkAsFailed は TransitionTo(FAILED) を呼び出すが、FAILED -> FAILED は無効な遷移であるため、
	// 堅牢性チェックのため、Statusをリセットして再試行可能な状態をシミュレートする。
	je.Status = model.BatchStatusStarted
	je.EndTime = nil

	// 同じエラーメッセージで MarkAsFailed を呼び出す
	je.MarkAsFailed(err1)
	assert.Equal(t, model.BatchStatusFailed, je.Status)
	assert.Len(t, je.Failures, 1) // 重複は追加されないこと

	// 3. 異なるエラーで MarkAsFailed を呼び出す
	err2 := errors.New("second critical failure")
	je.Status = model.BatchStatusStarted
	je.EndTime = nil
	je.MarkAsFailed(err2)
	assert.Equal(t, model.BatchStatusFailed, je.Status)
	assert.Len(t, je.Failures, 2) // 新しいエラーが追加されること
	assert.Contains(t, je.Failures, "critical system failure")
	assert.Contains(t, je.Failures, "second critical failure")
}

func TestStepExecution_TransitionTo(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusStarted)
	se := newTestStepExecution(je, model.BatchStatusStarting)

	// Test valid transitions
	assert.NoError(t, se.TransitionTo(model.BatchStatusStarted))
	assert.Equal(t, model.BatchStatusStarted, se.Status)

	assert.NoError(t, se.TransitionTo(model.BatchStatusCompleted))
	assert.Equal(t, model.BatchStatusCompleted, se.Status)

	// Test invalid transition
	err := se.TransitionTo(model.BatchStatusStarted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid state transition")
}

func TestStepExecution_AddFailureException_Deduplication(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusStarted)
	se := newTestStepExecution(je, model.BatchStatusStarted)
	err1 := errors.New("item read failed")
	err2 := errors.New("item read failed")
	err3 := errors.New("item write failed")

	se.AddFailureException(err1)
	assert.Len(t, se.Failures, 1)

	se.AddFailureException(err2) // Duplicate
	assert.Len(t, se.Failures, 1)

	se.AddFailureException(err3) // New error
	assert.Len(t, se.Failures, 2)
}

func TestJobParameters_Contains(t *testing.T) {
	fullParams := model.NewJobParameters()
	fullParams.Put("key1", "value1")
	fullParams.Put("key2", 123)
	fullParams.Put("key3", true)
	fullParams.Put("key4", 123.0) // float64として保存

	t.Run("EmptyPartial", func(t *testing.T) {
		partial := model.NewJobParameters()
		assert.True(t, fullParams.Contains(partial))
	})

	t.Run("FullMatch", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key1", "value1")
		partial.Put("key2", 123)
		assert.True(t, fullParams.Contains(partial))
	})

	t.Run("NumericTypeTolerance_IntToFloat", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key2", 123.0) // fullParams: int(123), partial: float64(123.0)
		assert.True(t, fullParams.Contains(partial))
	})

	t.Run("NumericTypeTolerance_FloatToInt", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key4", 123) // fullParams: float64(123.0), partial: int(123)
		assert.True(t, fullParams.Contains(partial))
	})

	t.Run("ValueMismatch", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key1", "valueX")
		assert.False(t, fullParams.Contains(partial))
	})

	t.Run("KeyMissing", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key5", "value5")
		assert.False(t, fullParams.Contains(partial))
	})

	t.Run("TypeMismatch_NonNumeric", func(t *testing.T) {
		partial := model.NewJobParameters()
		partial.Put("key2", "123") // int vs string (非数値は厳密比較)
		assert.False(t, fullParams.Contains(partial))
	})
}

func TestExecutionContext_PutNested_GetNested(t *testing.T) {
	ec := model.NewExecutionContext()

	// Set nested value
	ec.PutNested("reader_context.currentIndex", 10)
	ec.PutNested("reader_context.isFinished", false)
	ec.PutNested("topLevelKey", "test")

	// Get nested value
	val, ok := ec.GetNested("reader_context.currentIndex")
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	val, ok = ec.GetNested("reader_context.isFinished")
	assert.True(t, ok)
	assert.Equal(t, false, val)

	// Get top level value
	val, ok = ec.GetNested("topLevelKey")
	assert.True(t, ok)
	assert.Equal(t, "test", val)

	// Get non-existent path
	_, ok = ec.GetNested("non.existent")
	assert.False(t, ok)

	// Overwrite intermediate path (should warn but succeed)
	ec.Put("reader_context", "not_a_map")
	ec.PutNested("reader_context.newKey", "newVal") // This should overwrite "reader_context" first, then set "newKey"

	val, ok = ec.GetNested("reader_context.newKey")
	assert.True(t, ok)
	assert.Equal(t, "newVal", val)

	// Check if the intermediate path is now a map again
	intermediate, ok := ec.Get("reader_context")
	assert.True(t, ok)
	assert.IsType(t, model.NewExecutionContext(), intermediate)
}

func TestExecutionContext_GetTyped(t *testing.T) {
	ec := model.NewExecutionContext()
	ec.Put("strKey", "hello")
	ec.Put("intKey", 42)
	ec.Put("floatKey", 3.14)
	ec.Put("boolKey", true)
	ec.Put("wrongType", "42")

	// GetString
	s, ok := ec.GetString("strKey")
	assert.True(t, ok)
	assert.Equal(t, "hello", s)
	_, ok = ec.GetString("intKey")
	assert.False(t, ok)

	// GetInt
	i, ok := ec.GetInt("intKey")
	assert.True(t, ok)
	assert.Equal(t, 42, i)
	// Test float64 conversion (common when unmarshaling JSON)
	ec.Put("floatIntKey", 42.0)
	i, ok = ec.GetInt("floatIntKey")
	assert.True(t, ok)
	assert.Equal(t, 42, i)
	_, ok = ec.GetInt("wrongType")
	assert.False(t, ok)

	// GetBool
	b, ok := ec.GetBool("boolKey")
	assert.True(t, ok)
	assert.True(t, b)
	_, ok = ec.GetBool("intKey")
	assert.False(t, ok)

	// GetFloat64
	f, ok := ec.GetFloat64("floatKey")
	assert.True(t, ok)
	assert.Equal(t, 3.14, f)

	// If we rely on direct assignment:
	_, ok = ec.GetFloat64("intKey")
	assert.False(t, ok) // Fails because 42 is int, not float64
}

func TestJobParameters_GetTyped(t *testing.T) {
	jp := model.NewJobParameters()
	jp.Put("strKey", "hello")
	jp.Put("intKey", 42)
	jp.Put("floatKey", 3.14)
	jp.Put("boolKey", true)

	// GetString
	s, ok := jp.GetString("strKey")
	assert.True(t, ok)
	assert.Equal(t, "hello", s)

	// GetInt
	i, ok := jp.GetInt("intKey")
	assert.True(t, ok)
	assert.Equal(t, 42, i)

	// GetBool
	b, ok := jp.GetBool("boolKey")
	assert.True(t, ok)
	assert.True(t, b)

	// GetFloat64
	f, ok := jp.GetFloat64("floatKey")
	assert.True(t, ok)
	assert.Equal(t, 3.14, f)
}

func TestJobParameters_Hash(t *testing.T) {
	jp1 := model.NewJobParameters()
	jp1.Put("keyA", "value1")
	jp1.Put("keyB", 100)

	jp2 := model.NewJobParameters()
	jp2.Put("keyB", 100)
	jp2.Put("keyA", "value1") // Different order

	jp3 := model.NewJobParameters()
	jp3.Put("keyA", "valueX")

	hash1, err := jp1.Hash()
	assert.NoError(t, err)
	hash2, err := jp2.Hash()
	assert.NoError(t, err)
	hash3, err := jp3.Hash()
	assert.NoError(t, err)

	// Order independent check
	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)

	// Test nested map hashing (requires canonical JSON generation)
	jpNested1 := model.NewJobParameters()
	jpNested1.Put("z", map[string]interface{}{"b": 2, "a": 1})
	jpNested1.Put("a", 1)

	jpNested2 := model.NewJobParameters()
	jpNested2.Put("a", 1)
	jpNested2.Put("z", map[string]interface{}{"a": 1, "b": 2}) // Nested map order changed

	hashNested1, err := jpNested1.Hash()
	assert.NoError(t, err)
	hashNested2, err := jpNested2.Hash()
	assert.NoError(t, err)

	assert.Equal(t, hashNested1, hashNested2)
}

func TestJobExecution_IncrementRestartCount(t *testing.T) {
	je := newTestJobExecution(model.BatchStatusFailed)
	initialLastUpdated := je.LastUpdated
	assert.Equal(t, 0, je.RestartCount)

	time.Sleep(1 * time.Millisecond)
	je.IncrementRestartCount()
	assert.Equal(t, 1, je.RestartCount)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
	initialLastUpdated = je.LastUpdated

	time.Sleep(1 * time.Millisecond)
	je.IncrementRestartCount()
	assert.Equal(t, 2, je.RestartCount)
	assert.True(t, je.LastUpdated.After(initialLastUpdated))
}

func TestBatchError_IsErrorOfType_OptimisticLocking(t *testing.T) {
	// 楽観的ロックエラーのテスト
	olfe := exception.NewOptimisticLockingFailureException("repo", "update failed", nil)

	// 1. errors.Is によるチェック
	assert.True(t, errors.Is(olfe, exception.ErrOptimisticLockingFailure))

	// 2. IsErrorOfType によるチェック (Sentinel Name)
	assert.True(t, exception.IsErrorOfType(olfe, exception.OptimisticLockingFailureException))

	// 3. IsErrorOfType によるチェック (Wrapped Error Name)
	assert.True(t, exception.IsErrorOfType(olfe, "OptimisticLockingFailureException"))

	// 4. 異なるエラー
	otherErr := errors.New("some other error")
	assert.False(t, exception.IsErrorOfType(otherErr, exception.OptimisticLockingFailureException))
}
