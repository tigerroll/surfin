package model

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewID generates a new unique identifier.
func NewID() string {
	return uuid.New().String()
}

// PartitionName generates a partition name string from an index.
func PartitionName(i int) string {
	return fmt.Sprintf("partition-%d", i)
}

// ToUnixMillisPtr converts a time.Time to a pointer to UnixMillis.
// If the input time is a zero value, it returns nil.
func ToUnixMillisPtr(t time.Time) *UnixMillis {
	if t.IsZero() {
		return nil
	}
	um := UnixMillis(t.UnixMilli())
	return &um
}

// UnixMillis is a custom type for storing Unix timestamps in milliseconds.
// It implements the sql.Scanner and driver.Valuer interfaces to handle conversions
// between time.Time (from database) and int64 (for Parquet/internal representation).
type UnixMillis int64

// Scan implements the sql.Scanner interface.
// It converts a value read from the database into a UnixMillis type.
func (um *UnixMillis) Scan(value interface{}) error {
	if value == nil {
		*um = 0 // Represent NULL as 0 for optional int64.
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		*um = UnixMillis(v.UnixMilli())
		return nil
	case int64: // If the database already returns int64
		*um = UnixMillis(v)
		return nil
	case string: // Handle cases where SQLite might return string for INTEGER column
		// 1. Try parsing with timezone offset (e.g., "2019-02-11 09:00:00+09:00")
		parsedTime, err := time.Parse("2006-01-02 15:04:05-07:00", v)
		if err == nil {
			*um = UnixMillis(parsedTime.UnixMilli())
			return nil
		}

		// 2. If that fails, try parsing without timezone offset (e.g., "2019-02-11 09:00:00")
		parsedTime, err = time.Parse("2006-01-02 15:04:05", v)
		if err == nil {
			*um = UnixMillis(parsedTime.UnixMilli())
			return nil
		}

		// 3. If that also fails, try parsing date only (e.g., "2019-02-11")
		parsedTime, err = time.Parse("2006-01-02", v)
		if err == nil {
			*um = UnixMillis(parsedTime.UnixMilli())
			return nil
		}

		// If all parsing fails, return an error
		return fmt.Errorf("cannot scan string '%s' into UnixMillis: failed to parse with known layouts", v)

	default:
		return fmt.Errorf("cannot scan type %T into UnixMillis", v)
	}
}

// Value implements the driver.Valuer interface.
// It converts a UnixMillis value into a driver.Value for writing to the database.
// If the UnixMillis value is 0, it returns time.UnixMilli(0) (epoch start) instead of nil,
// to satisfy NOT NULL constraints in the database.
func (um UnixMillis) Value() (driver.Value, error) {
	// If um is 0, return the Unix epoch start time (1970-01-01 00:00:00 UTC)
	// instead of nil, to satisfy NOT NULL constraints.
	return time.UnixMilli(int64(um)), nil
}

// JobStatus represents the state of a job execution.
type JobStatus string

const (
	BatchStatusStarting       JobStatus = "STARTING"
	BatchStatusStarted        JobStatus = "STARTED"
	BatchStatusStopping       JobStatus = "STOPPING"
	BatchStatusStopped        JobStatus = "STOPPED"
	BatchStatusCompleted      JobStatus = "COMPLETED"
	BatchStatusFailed         JobStatus = "FAILED"
	BatchStatusAbandoned      JobStatus = "ABANDONED"
	BatchStatusCompleting     JobStatus = "COMPLETING"
	BatchStatusStoppingFailed JobStatus = "STOPPING_FAILED"
	BatchStatusRestarting     JobStatus = "RESTARTING"
	BatchStatusUnknown        JobStatus = "UNKNOWN"
)

// String returns the string representation of the JobStatus.
func (s JobStatus) String() string {
	return string(s)
}

// IsFinished checks if the JobStatus represents a finished state.
func (s JobStatus) IsFinished() bool {
	switch s {
	case BatchStatusCompleted, BatchStatusFailed, BatchStatusStopped, BatchStatusAbandoned:
		return true
	default:
		return false
	}
}

// ToExitStatus converts the JobStatus to its corresponding ExitStatus.
func (s JobStatus) ToExitStatus() ExitStatus {
	switch s {
	case BatchStatusCompleted:
		return ExitStatusCompleted
	case BatchStatusFailed:
		return ExitStatusFailed
	case BatchStatusStopped:
		return ExitStatusStopped
	case BatchStatusAbandoned:
		return ExitStatusAbandoned
	default:
		return ExitStatusUnknown
	}
}

// ExitStatus represents the detailed status upon job/step completion.
type ExitStatus string

const (
	ExitStatusUnknown   ExitStatus = "UNKNOWN"
	ExitStatusCompleted ExitStatus = "COMPLETED"
	ExitStatusFailed    ExitStatus = "FAILED"
	ExitStatusStopped   ExitStatus = "STOPPED"
	ExitStatusAbandoned ExitStatus = "ABANDONED"
	ExitStatusNoOp      ExitStatus = "NO_OP"
)

// String returns the ExitStatus as a string.
func (s ExitStatus) String() string {
	return string(s)
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState string

const (
	CBStateClosed   CircuitBreakerState = "CLOSED"
	CBStateOpen     CircuitBreakerState = "OPEN"
	CBStateHalfOpen CircuitBreakerState = "HALF_OPEN"
)

// ExecutionContext is a key-value store for sharing state across job and step executions.
type ExecutionContext map[string]interface{}

// Value implements the `driver.Valuer` interface, converting the ExecutionContext to a JSON string.
func (ec ExecutionContext) Value() (driver.Value, error) {
	if ec == nil {
		return "{}", nil // Return empty map as JSON
	}
	data, err := json.Marshal(ec)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan implements the `sql.Scanner` interface, converting a JSON string to an ExecutionContext.
func (ec *ExecutionContext) Scan(value interface{}) error {
	if value == nil {
		*ec = make(ExecutionContext)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte: // Handle byte slice from database
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan type for ExecutionContext: %T", value)
	}

	if len(b) == 0 {
		*ec = make(ExecutionContext)
		return nil // Return empty map if the byte slice is empty
	}

	// JSON decode
	if err := json.Unmarshal(b, ec); err != nil {
		return fmt.Errorf("failed to unmarshal ExecutionContext JSON: %w", err)
	}
	return nil
}

// Copy creates a shallow copy of the ExecutionContext.
func (ec ExecutionContext) Copy() ExecutionContext {
	newEC := NewExecutionContext()
	for k, v := range ec {
		newEC[k] = v
	}
	return newEC
}

// Remove deletes a key from the ExecutionContext.
func (ec ExecutionContext) Remove(key string) {
	delete(ec, key)
}

// PutNested sets a value in a nested map structure within the ExecutionContext.
func (ec ExecutionContext) PutNested(key string, value interface{}) {
	if !strings.Contains(key, ".") {
		ec[key] = value
		return
	}

	parts := strings.Split(key, ".")
	current := ec
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			newMap := NewExecutionContext()
			current[part] = newMap
			current = newMap
		} else {
			if nextMap, ok := next.(ExecutionContext); ok {
				current = nextMap
			} else {
				newMap := NewExecutionContext()
				current[part] = newMap
				current = newMap
			}
		}
	}
	current[parts[len(parts)-1]] = value
}

// GetNested retrieves a value from a nested map structure.
func (ec ExecutionContext) GetNested(key string) (interface{}, bool) {
	if val, ok := ec[key]; ok {
		return val, true
	}

	parts := strings.Split(key, ".")
	current := ec

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return nil, false
		}

		// If not the last part, we expect the value to be an ExecutionContext
		if i < len(parts)-1 {
			next, ok := val.(ExecutionContext)
			if !ok {
				return nil, false
			}
			current = next
		} else {
			// Last part, return the value
			return val, true
		}
	}
	return nil, false
}

// Get retrieves a value from the ExecutionContext.
func (ec ExecutionContext) Get(key string) (interface{}, bool) {
	val, ok := ec[key]
	return val, ok
}

// Put sets a value in the ExecutionContext.
func (ec ExecutionContext) Put(key string, value interface{}) {
	ec[key] = value
}

// GetString retrieves a string value from the ExecutionContext.
func (ec ExecutionContext) GetString(key string) (string, bool) {
	val, ok := ec[key]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// GetInt retrieves an int value from the ExecutionContext.
func (ec ExecutionContext) GetInt(key string) (int, bool) {
	val, ok := ec[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool retrieves a bool value from the ExecutionContext.
func (ec ExecutionContext) GetBool(key string) (bool, bool) {
	val, ok := ec[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetFloat64 retrieves a float64 value from the ExecutionContext.
func (ec ExecutionContext) GetFloat64(key string) (float64, bool) {
	val, ok := ec[key]
	if !ok {
		return 0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// JobParameters is a structure holding parameters for job execution.
type JobParameters struct {
	Params map[string]interface{}
}

// Value implements the `driver.Valuer` interface, converting JobParameters to a JSON string.
func (jp JobParameters) Value() (driver.Value, error) {
	if jp.Params == nil {
		return "{}", nil
	}
	data, err := json.Marshal(jp.Params)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan implements the `sql.Scanner` interface, converting a JSON string to JobParameters.
func (jp *JobParameters) Scan(value interface{}) error {
	if value == nil {
		jp.Params = make(map[string]interface{})
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte: // Handle byte slice from database
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan type for JobParameters: %T", value)
	}

	if len(b) == 0 {
		jp.Params = make(map[string]interface{})
		return nil // Return empty map if the byte slice is empty
	}

	// JSON decode
	if err := json.Unmarshal(b, &jp.Params); err != nil {
		return fmt.Errorf("failed to unmarshal JobParameters JSON: %w", err)
	}
	return nil
}

// Hash generates a hash for the JobParameters.
func (jp JobParameters) Hash() (string, error) {
	data, err := json.Marshal(jp.Params)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", data), nil
}

// Equal checks if two JobParameters are equal.
func (jp JobParameters) Equal(other JobParameters) bool {
	return fmt.Sprintf("%v", jp.Params) == fmt.Sprintf("%v", other.Params)
}

// Contains checks if the current JobParameters contains all parameters from the other.
func (jp JobParameters) Contains(other JobParameters) bool {
	for k, v := range other.Params {
		val, ok := jp.Params[k]
		if !ok {
			return false
		}

		// Compare numeric types by converting to float64.
		f1, ok1 := toFloat64(val)
		f2, ok2 := toFloat64(v)

		if ok1 && ok2 {
			if f1 == f2 {
				continue
			}
			return false
		}

		// Compare non-numeric types directly.
		if val != v {
			return false
		}
	}
	return true
}

// toFloat64 is a helper to convert numeric types to float64 for comparison.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}

// GetString retrieves a string value from the JobParameters.
func (jp JobParameters) GetString(key string) (string, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// GetInt retrieves an int value from the JobParameters.
func (jp JobParameters) GetInt(key string) (int, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool retrieves a bool value from the JobParameters.
func (jp JobParameters) GetBool(key string) (bool, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool) // Use val instead of accessing the map directly.
	return b, ok
}

// GetFloat64 retrieves a float64 value from the JobParameters.
func (jp JobParameters) GetFloat64(key string) (float64, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return 0, false
	}
	f, ok := val.(float64) // Use val instead of accessing the map directly.
	return f, ok
}

// Put sets a value in the JobParameters.
func (jp JobParameters) Put(key string, value interface{}) {
	if jp.Params == nil {
		jp.Params = make(map[string]interface{})
	}
	jp.Params[key] = value
}

// String returns the JSON string representation of the JobParameters.
func (jp JobParameters) String() string {
	data, _ := json.Marshal(jp.Params)
	return string(data)
}

// JobInstance represents a unique job instance.
type JobInstance struct {
	ID             string
	JobName        string
	Parameters     JobParameters
	ParametersHash string
	Version        int
	CreateTime     time.Time
}

// JobExecution represents a single execution of a JobInstance.
type JobExecution struct {
	ID               string
	JobInstanceID    string
	JobName          string
	Status           JobStatus
	ExitStatus       ExitStatus
	ExitCode         int // Exit code of the job execution.
	StartTime        time.Time
	EndTime          *time.Time
	CreateTime       time.Time // Creation time of the job execution.
	LastUpdated      time.Time
	ExecutionContext ExecutionContext
	Parameters       JobParameters
	Failures         FailureList
	Version          int
	RestartCount     int
	StepExecutions   []*StepExecution
	CurrentStepName  string
	CancelFunc       context.CancelFunc // Function to cancel the job execution.
}

// StepExecution represents a single execution of a Step.
type StepExecution struct {
	ID               string
	StepName         string
	JobExecutionID   string
	JobExecution     *JobExecution
	Status           JobStatus
	ExitStatus       ExitStatus
	StartTime        time.Time
	EndTime          *time.Time
	LastUpdated      time.Time
	ExecutionContext ExecutionContext
	ReadCount        int
	WriteCount       int
	CommitCount      int
	RollbackCount    int
	FilterCount      int
	SkipReadCount    int
	SkipProcessCount int
	SkipWriteCount   int
	Failures         FailureList
	Version          int
}

// CopyForRestart creates a copy of the StepExecution for restart purposes.
func (se *StepExecution) CopyForRestart(newID string) *StepExecution {
	return &StepExecution{
		ID:               newID,
		StepName:         se.StepName,
		ExecutionContext: se.ExecutionContext.Copy(),
		// Copy other fields as needed.
	}
}

// CheckpointData represents the state of a step execution for checkpointing.
type CheckpointData struct {
	StepExecutionID  string
	ExecutionContext ExecutionContext
	LastUpdated      time.Time
}

// ExecutionContextPromotion defines how to promote execution context.
type ExecutionContextPromotion struct {
	Keys         []string
	JobLevelKeys map[string]string
}

// FailureList is a list of error messages.
type FailureList []string

// Value implements the driver.Valuer interface for FailureList.
func (fl FailureList) Value() (driver.Value, error) {
	if fl == nil {
		return "[]", nil
	}
	data, err := json.Marshal(fl)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan implements the sql.Scanner interface for FailureList.
func (fl *FailureList) Scan(value interface{}) error {
	if value == nil {
		*fl = FailureList{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan type for FailureList: %T", value)
	}
	return json.Unmarshal(b, fl)
}

// NewJobExecution creates a new JobExecution instance.
func NewJobExecution(jobInstanceID string, jobName string, params JobParameters) *JobExecution {
	return &JobExecution{
		ID:               NewID(),       // Generate a new ID.
		JobInstanceID:    jobInstanceID, // Set the foreign key.
		JobName:          jobName,
		Parameters:       params,
		Status:           BatchStatusStarting,
		ExitStatus:       ExitStatusUnknown,
		StartTime:        time.Now(),
		CreateTime:       time.Now(),
		ExecutionContext: NewExecutionContext(),
		Failures:         FailureList{},
	}
}

// NewJobInstance creates a new JobInstance instance.
func NewJobInstance(jobName string, params JobParameters) *JobInstance {
	hash, _ := params.Hash()
	return &JobInstance{
		ID:             NewID(),
		JobName:        jobName,
		Parameters:     params,
		ParametersHash: hash,
		CreateTime:     time.Now(),
	}
}

// NewExecutionContext creates a new empty ExecutionContext.
func NewExecutionContext() ExecutionContext {
	return make(ExecutionContext)
}

// NewStepExecution creates a new StepExecution instance.
func NewStepExecution(id string, jobExec *JobExecution, stepName string) *StepExecution {
	return &StepExecution{
		ID:               id,
		JobExecution:     jobExec,
		JobExecutionID:   jobExec.ID,
		StepName:         stepName,
		Status:           BatchStatusStarting,
		ExitStatus:       ExitStatusUnknown,
		StartTime:        time.Now(),
		ExecutionContext: NewExecutionContext(),
		Failures:         FailureList{},
	}
}

// NewJobParameters creates a new JobParameters instance.
func NewJobParameters() JobParameters {
	return JobParameters{Params: make(map[string]interface{})}
}

// NewExecutionContextPromotion creates a new ExecutionContextPromotion instance.
func NewExecutionContextPromotion() *ExecutionContextPromotion {
	return &ExecutionContextPromotion{
		Keys:         []string{},
		JobLevelKeys: make(map[string]string),
	}
}

// IncrementRestartCount increments the restart count for the JobExecution.
func (je *JobExecution) IncrementRestartCount() {
	je.RestartCount++
	je.LastUpdated = time.Now()
}

// TransitionTo transitions the JobExecution to a new status.
func (je *JobExecution) TransitionTo(status JobStatus) error {
	if !isValidJobTransition(je.Status, status) {
		return fmt.Errorf("Invalid state transition: %s -> %s", je.Status, status)
	}
	je.Status = status
	je.LastUpdated = time.Now()
	return nil
}

// TransitionTo transitions the StepExecution to a new status.
func (se *StepExecution) TransitionTo(status JobStatus) error {
	if !isValidStepTransition(se.Status, status) {
		return fmt.Errorf("Invalid state transition: %s -> %s", se.Status, status)
	}
	se.Status = status
	se.LastUpdated = time.Now()
	return nil
}

func isValidJobTransition(from, to JobStatus) bool {
	switch from {
	case BatchStatusStarting:
		return to == BatchStatusStarted || to == BatchStatusFailed
	case BatchStatusStarted:
		return to == BatchStatusStopping || to == BatchStatusCompleted || to == BatchStatusFailed
	case BatchStatusStopping:
		return to == BatchStatusStopped
	case BatchStatusStopped:
		return to == BatchStatusRestarting
	case BatchStatusRestarting:
		return to == BatchStatusStarted || to == BatchStatusFailed
	case BatchStatusFailed:
		return to == BatchStatusAbandoned || to == BatchStatusRestarting
	default:
		return false
	}
}

func isValidStepTransition(from, to JobStatus) bool {
	switch from {
	case BatchStatusStarting:
		return to == BatchStatusStarted || to == BatchStatusFailed
	case BatchStatusStarted:
		return to == BatchStatusCompleted || to == BatchStatusFailed || to == BatchStatusStopped
	case BatchStatusStopped:
		return to == BatchStatusStarted
	default:
		return false
	}
}

// MarkAsStarted marks the JobExecution as started.
func (je *JobExecution) MarkAsStarted() {
	je.Status = BatchStatusStarted
	je.LastUpdated = time.Now()
}

// MarkAsCompleted marks the JobExecution as completed.
func (je *JobExecution) MarkAsCompleted() {
	je.Status = BatchStatusCompleted
	je.ExitStatus = ExitStatusCompleted
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// MarkAsFailed marks the JobExecution as failed.
func (je *JobExecution) MarkAsFailed(err error) {
	je.Status = BatchStatusFailed
	je.ExitStatus = ExitStatusFailed
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
	je.AddFailureException(err)
}

// MarkAsStopped marks the JobExecution as stopped.
func (je *JobExecution) MarkAsStopped() {
	je.Status = BatchStatusStopped
	je.ExitStatus = ExitStatusStopped
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// MarkAsAbandoned marks the JobExecution as abandoned.
func (je *JobExecution) MarkAsAbandoned() {
	je.Status = BatchStatusAbandoned
	je.ExitStatus = ExitStatusAbandoned
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// AddFailureException adds a failure exception to the JobExecution.
func (je *JobExecution) AddFailureException(err error) {
	msg := err.Error()
	for _, f := range je.Failures {
		if f == msg {
			return
		}
	}
	je.Failures = append(je.Failures, msg)
}

// AddStepExecution adds a StepExecution to the JobExecution.
func (je *JobExecution) AddStepExecution(se *StepExecution) {
	je.StepExecutions = append(je.StepExecutions, se)
}

// AddFailureException adds a failure exception to the StepExecution.
func (se *StepExecution) AddFailureException(err error) {
	msg := err.Error()
	for _, f := range se.Failures {
		if f == msg {
			return
		}
	}
	se.Failures = append(se.Failures, msg)
}

// MarkAsStarted marks the StepExecution as started.
func (se *StepExecution) MarkAsStarted() {
	se.Status = BatchStatusStarted
	se.LastUpdated = time.Now()
}

// MarkAsCompleted marks the StepExecution as completed.
func (se *StepExecution) MarkAsCompleted() {
	se.Status = BatchStatusCompleted
	se.ExitStatus = ExitStatusCompleted
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
}

// MarkAsFailed marks the StepExecution as failed.
func (se *StepExecution) MarkAsFailed(err error) {
	se.Status = BatchStatusFailed
	se.ExitStatus = ExitStatusFailed
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
	se.AddFailureException(err)
}

// MarkAsStopped marks the StepExecution as stopped.
func (se *StepExecution) MarkAsStopped() {
	se.Status = BatchStatusStopped
	se.ExitStatus = ExitStatusStopped
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
}

// DebugString returns a debug string representation of the StepExecution.
func (se *StepExecution) DebugString() string {
	return fmt.Sprintf("StepExecution[ID=%s, Name=%s, Status=%s]", se.ID, se.StepName, se.Status)
}

// FlowDefinition defines the flow of a job.
type FlowDefinition struct {
	StartElement string
	Elements     map[string]interface{}
	Transitions  map[string][]TransitionRule
}

// TransitionRule defines a transition rule in the flow.
type TransitionRule struct {
	Transition Transition
}

// Transition defines the transition logic.
type Transition struct {
	To   string
	End  bool
	Fail bool
	Stop bool
}

// NewFlowDefinition creates a new FlowDefinition instance.
func NewFlowDefinition(start string) *FlowDefinition {
	return &FlowDefinition{
		StartElement: start,
		Elements:     make(map[string]interface{}),
		Transitions:  make(map[string][]TransitionRule),
	}
}

// AddElement adds an element to the flow.
func (fd *FlowDefinition) AddElement(id string, element interface{}) error {
	if _, exists := fd.Elements[id]; exists {
		return fmt.Errorf("element with ID '%s' already exists", id)
	}
	fd.Elements[id] = element
	return nil
}

// AddTransitionRule adds a transition rule to the flow.
func (fd *FlowDefinition) AddTransitionRule(from string, status string, to string, end, fail, stop bool) error {
	fd.Transitions[from] = append(fd.Transitions[from], TransitionRule{
		Transition: Transition{To: to, End: end, Fail: fail, Stop: stop},
	})
	return nil
}

// GetTransitionRule retrieves a transition rule.
func (fd *FlowDefinition) GetTransitionRule(from string, status ExitStatus, isError bool) (TransitionRule, bool) {
	rules, ok := fd.Transitions[from]
	if !ok || len(rules) == 0 {
		return TransitionRule{}, false
	}
	// Simple implementation: return the first rule.
	return rules[0], true
}
