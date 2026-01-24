package model

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	logger "github.com/tigerroll/surfin/pkg/batch/support/util/logger"
	"github.com/tigerroll/surfin/pkg/batch/support/util/serialization"

	"github.com/google/uuid"
)

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

// FailureList holds a list of error messages.
type FailureList []string

// Value implements the `driver.Valuer` interface, converting FailureList to a JSON string.
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

// Scan implements the `sql.Scanner` interface, converting a JSON string to FailureList.
func (fl *FailureList) Scan(value interface{}) error {
	if value == nil {
		*fl = make(FailureList, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte: // Handle byte slice from database
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan type for FailureList: %T", value)
	}

	if len(b) == 0 {
		*fl = make(FailureList, 0)
		return nil // Return empty list if the byte slice is empty
	}

	// JSON decode
	if err := json.Unmarshal(b, fl); err != nil {
		return fmt.Errorf("failed to unmarshal FailureList JSON: %w", err)
	}
	return nil
}

// JobInstance is a structure representing the logical execution unit of a job.
type JobInstance struct {
	ID             string
	JobName        string
	Parameters     JobParameters
	CreateTime     time.Time
	Version        int
	ParametersHash string
}

// JobExecution is a structure representing a single execution instance of a job.
type JobExecution struct {
	ID               string
	JobInstanceID    string
	JobName          string
	Parameters       JobParameters
	StartTime        time.Time
	EndTime          *time.Time
	Status           JobStatus
	ExitStatus       ExitStatus
	ExitCode         int
	Failures         FailureList
	Version          int
	CreateTime       time.Time
	LastUpdated      time.Time
	StepExecutions   []*StepExecution
	ExecutionContext ExecutionContext
	CurrentStepName  string
	CancelFunc       context.CancelFunc
	RestartCount     int
}

// StepExecution is a structure representing a single execution instance of a step.
type StepExecution struct {
	ID               string
	StepName         string
	JobExecution     *JobExecution
	StartTime        time.Time
	JobExecutionID   string
	EndTime          *time.Time
	Status           JobStatus
	ExitStatus       ExitStatus
	Failures         FailureList
	ReadCount        int
	WriteCount       int
	CommitCount      int
	RollbackCount    int
	FilterCount      int
	SkipReadCount    int
	SkipProcessCount int
	SkipWriteCount   int
	ExecutionContext ExecutionContext
	LastUpdated      time.Time
	Version          int
}

// CopyForRestart creates a deep copy of the StepExecution for a restart attempt. It generates a new ID and resets the status if the original status is not COMPLETED.
func (se *StepExecution) CopyForRestart(newJobExecutionID string) *StepExecution {
	newSE := &StepExecution{
		ID:               NewID(), // Generate a new ID for the restart attempt
		StepName:         se.StepName,
		JobExecutionID:   newJobExecutionID,
		JobExecution:     nil, // Will be set by the caller (JobExecution)
		Failures:         FailureList{},
		ExecutionContext: NewExecutionContext(),
		Version:          0,
	}

	for k, v := range se.ExecutionContext {
		newSE.ExecutionContext[k] = v
	} // Copy existing ExecutionContext

	// Only copy status/time/stats if the step was completed successfully in the previous execution
	if se.Status == BatchStatusCompleted {
		newSE.Status = BatchStatusCompleted
		newSE.ExitStatus = se.ExitStatus
		newSE.StartTime = se.StartTime
		newSE.EndTime = se.EndTime
		newSE.ReadCount = se.ReadCount
		newSE.WriteCount = se.WriteCount
		newSE.CommitCount = se.CommitCount
		newSE.RollbackCount = se.RollbackCount
		newSE.FilterCount = se.FilterCount
		newSE.SkipReadCount = se.SkipReadCount
		newSE.SkipProcessCount = se.SkipProcessCount
		newSE.SkipWriteCount = se.SkipWriteCount
	} else {
		// For steps that failed or were stopped, reset status to Starting and clear stats
		newSE.Status = BatchStatusStarting
		newSE.ExitStatus = ExitStatusUnknown
		newSE.StartTime = time.Now() // Set new start time upon creation for the restarted step
		newSE.EndTime = nil          // Reset end time for the restarted step
		// Stats remain 0 (default) for the restarted step
	}

	return newSE
}

// DebugString returns a debug string representation of StepExecution, excluding ExecutionContext details.
func (se *StepExecution) DebugString() string {
	endTimeStr := "nil"
	if se.EndTime != nil {
		endTimeStr = se.EndTime.Format(time.RFC3339Nano)
	}

	return fmt.Sprintf(
		"&{ID:%s StepName:%s JobExecutionID:%s StartTime:%s EndTime:%s Status:%s ExitStatus:%s Failures:%v ReadCount:%d WriteCount:%d CommitCount:%d RollbackCount:%d FilterCount:%d SkipReadCount:%d SkipProcessCount:%d SkipWriteCount:%d ExecutionContext: (omitted, size: %d) LastUpdated:%s Version:%d}",
		se.ID, se.StepName, se.JobExecutionID, se.StartTime.Format(time.RFC3339Nano),
		endTimeStr, se.Status, se.ExitStatus, se.Failures,
		se.ReadCount, se.WriteCount, se.CommitCount, se.RollbackCount, se.FilterCount,
		se.SkipReadCount, se.SkipProcessCount, se.SkipWriteCount, len(se.ExecutionContext),
		se.LastUpdated.Format(time.RFC3339Nano), se.Version,
	)
}

// CheckpointData is a structure for persisting StepExecution checkpoint data.
type CheckpointData struct {
	StepExecutionID  string
	ExecutionContext ExecutionContext
	LastUpdated      time.Time
}

// Transition defines a transition rule from a step or Decision to the next element.
type Transition struct {
	On   string `yaml:"on"`
	To   string `yaml:"to,omitempty"`
	End  bool   `yaml:"end,omitempty"`
	Fail bool   `yaml:"fail,omitempty"`
	Stop bool   `yaml:"stop,omitempty"`
}

// FlowDefinition defines the entire execution flow of a job.
type FlowDefinition struct {
	StartElement    string
	Elements        map[string]interface{} // interface{} is used here to avoid circular dependency with port.FlowElement
	TransitionRules []TransitionRule
}

// TransitionRule defines a single transition rule from a specific source element.
type TransitionRule struct {
	From       string
	Transition Transition
}

// NewFlowDefinition creates a new instance of FlowDefinition.
func NewFlowDefinition(startElement string) *FlowDefinition {
	return &FlowDefinition{
		StartElement:    startElement,
		Elements:        make(map[string]interface{}),
		TransitionRules: make([]TransitionRule, 0),
	}
}

// AddElement adds a new element (Step or Decision) to the flow.
func (fd *FlowDefinition) AddElement(id string, element interface{}) error {
	if _, exists := fd.Elements[id]; exists {
		return fmt.Errorf("flow element ID '%s' already exists", id)
	}
	fd.Elements[id] = element
	return nil
}

// AddTransitionRule adds a transition rule.
func (fd *FlowDefinition) AddTransitionRule(from, on, to string, end, fail, stop bool) {
	fd.TransitionRules = append(fd.TransitionRules, TransitionRule{
		From: from,
		Transition: Transition{
			On:   on,
			To:   to,
			End:  end,
			Fail: fail,
			Stop: stop,
		},
	})
}

// GetTransitionRule searches for the next transition rule based on the ExitStatus.
func (fd *FlowDefinition) GetTransitionRule(from string, exitStatus ExitStatus, hasError bool) (TransitionRule, bool) {
	for _, rule := range fd.TransitionRules {
		if rule.From == from && (rule.Transition.On == string(exitStatus) || rule.Transition.On == "*") {
			return rule, true
		}
	}
	return TransitionRule{}, false
}

// ExecutionContextPromotion defines the promotion settings from StepExecutionContext to JobExecutionContext.
type ExecutionContextPromotion struct {
	Keys []string `yaml:"keys,omitempty"`
	// JobLevelKeys is a map for renaming promoted keys at the job level.
	JobLevelKeys map[string]string `yaml:"job-level-keys,omitempty"`
}

// NewExecutionContextPromotion creates a new instance of ExecutionContextPromotion.
func NewExecutionContextPromotion() *ExecutionContextPromotion {
	return &ExecutionContextPromotion{
		Keys:         make([]string, 0),
		JobLevelKeys: make(map[string]string),
	}
}

// NewExecutionContext creates a new empty ExecutionContext.
func NewExecutionContext() ExecutionContext {
	return make(ExecutionContext)
}

// Put sets a value in the ExecutionContext with the specified key and value.
func (ec ExecutionContext) Put(key string, value interface{}) {
	ec[key] = value
}

// Get retrieves the value for the specified key. Returns nil and false if the value does not exist.
func (ec ExecutionContext) Get(key string) (interface{}, bool) {
	val, ok := ec[key]
	return val, ok
}

// GetString retrieves the value for the specified key as a string.
func (ec ExecutionContext) GetString(key string) (string, bool) {
	val, ok := ec[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves the value for the specified key as an int.
func (ec ExecutionContext) GetInt(key string) (int, bool) {
	val, ok := ec[key]
	if !ok {
		return 0, false
	}
	// Handle numbers unmarshaled from JSON which might be float64
	if i, ok := val.(int); ok {
		return i, true // Value is already an int
	}
	if f, ok := val.(float64); ok {
		return int(f), true
	}
	return 0, false
}

// GetBool retrieves the value for the specified key as a bool.
func (ec ExecutionContext) GetBool(key string) (bool, bool) {
	val, ok := ec[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetFloat64 retrieves the value for the specified key as a float64.
func (ec ExecutionContext) GetFloat64(key string) (float64, bool) {
	val, ok := ec[key]
	if !ok {
		return 0.0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// Copy creates a shallow copy of the ExecutionContext.
func (ec ExecutionContext) Copy() ExecutionContext {
	newEC := make(ExecutionContext, len(ec))
	for k, v := range ec {
		newEC[k] = v
	}
	return newEC
}

// GetNested retrieves a nested value using a dot-separated key.
// Example: "reader_context.currentIndex"
func (ec ExecutionContext) GetNested(key string) (interface{}, bool) {
	parts := strings.Split(key, ".")

	// 1. First, check if the entire key exists at the top level (e.g., if "p1.count" is the key name)
	if val, ok := ec.Get(key); ok {
		return val, true
	}

	var currentMap interface{} = ec
	var ok bool = true

	for i, part := range parts {
		if !ok {
			return nil, false // Path segment not found
		}

		// Ensure currentMap is map[string]interface{} or ExecutionContext
		var nextMap map[string]interface{}
		if m, isEC := currentMap.(ExecutionContext); isEC {
			nextMap = m
		} else if m, isMap := currentMap.(map[string]interface{}); isMap {
			nextMap = m
		} else {
			return nil, false // Current value is not a map, cannot descend further
		}

		currentMap, ok = nextMap[part]
		if !ok {
			return nil, false
		}

		// If not the last part, expect the next value to be a map
		if i < len(parts)-1 {
			if _, isMap := currentMap.(ExecutionContext); !isMap {
				if _, isMap := currentMap.(map[string]interface{}); !isMap {
					return nil, false // Current value is not a map, cannot descend further
				}
			}
		}
	}
	return currentMap, ok
}

// PutNested sets a value in the ExecutionContext using a nested key. Intermediate maps are created if they do not exist.
func (ec ExecutionContext) PutNested(key string, value interface{}) {
	parts := strings.Split(key, ".")
	currentMap := ec

	for i, part := range parts {
		if i == len(parts)-1 {
			currentMap[part] = value
		} else {
			nextMapVal, ok := currentMap[part]
			var nextMap ExecutionContext
			if !ok {
				nextMap = NewExecutionContext()
				currentMap[part] = nextMap
			} else {
				// Ensure the existing value is ExecutionContext or map[string]interface{}
				if m, isEC := nextMapVal.(ExecutionContext); isEC {
					nextMap = m
				} else if m, isMap := nextMapVal.(map[string]interface{}); isMap {
					nextMap = ExecutionContext(m) // Convert map[string]interface{} to ExecutionContext
				} else {
					// If the existing value is not a map, overwrite and create a new map
					logger.Warnf("ExecutionContext.PutNested: Overwriting existing non-map value at path '%s'.", strings.Join(parts[:i+1], "."))
					nextMap = NewExecutionContext()
					currentMap[part] = nextMap
				}
			}
			currentMap = nextMap
		}
	}
}

// Remove removes the specified key from the ExecutionContext.
func (ec ExecutionContext) Remove(key string) {
	delete(ec, key)
}

// NewJobParameters creates a new instance of JobParameters.
func NewJobParameters() JobParameters {
	return JobParameters{
		Params: make(map[string]interface{}),
	}
}

// Put sets a value in JobParameters with the specified key and value.
func (jp JobParameters) Put(key string, value interface{}) {
	jp.Params[key] = value
}

// Get retrieves the value for the specified key. Returns nil if the value does not exist.
func (jp JobParameters) Get(key string) interface{} {
	val, ok := jp.Params[key]
	if !ok {
		return nil
	}
	return val
}

// GetString retrieves the value for the specified key as a string.
func (jp JobParameters) GetString(key string) (string, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves the value for the specified key as an int.
func (jp JobParameters) GetInt(key string) (int, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return 0, false
	}
	// Handle numbers unmarshaled from JSON which might be float64
	if i, ok := val.(int); ok {
		return i, true
	}
	if f, ok := val.(float64); ok {
		return int(f), true
	}
	return 0, false
}

// GetBool retrieves the value for the specified key as a bool.
func (jp JobParameters) GetBool(key string) (bool, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetFloat64 retrieves the value for the specified key as a float64.
func (jp JobParameters) GetFloat64(key string) (float64, bool) {
	val, ok := jp.Params[key]
	if !ok {
		return 0.0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// Equal compares if two JobParameters are equal.
func (jp JobParameters) Equal(other JobParameters) bool {
	return reflect.DeepEqual(jp.Params, other.Params)
}

// Contains compares whether this JobParameters holds all keys and values of the specified partialParams.
func (jp JobParameters) Contains(partialParams JobParameters) bool {
	if len(partialParams.Params) == 0 {
		return true
	}
	for key, partialValue := range partialParams.Params {
		actualValue, ok := jp.Params[key]
		if !ok {
			return false // Key does not exist
		}

		// Compare if values match using custom comparison function
		if !deepEqualWithNumericTolerance(actualValue, partialValue) {
			return false // Values do not match
		}
	}
	return true
}

// deepEqualWithNumericTolerance returns true if values are equal, ignoring type differences for numeric types (int, float64). Otherwise, it delegates to reflect.DeepEqual.
func deepEqualWithNumericTolerance(a, b interface{}) bool {
	// Check if both are numeric types
	aIsNum := isNumeric(a)
	bIsNum := isNumeric(b) // Check if b is a numeric type

	if aIsNum && bIsNum {
		// Convert both to float64 and compare
		aFloat := toFloat64(a)
		bFloat := toFloat64(b)
		return aFloat == bFloat
	}

	// Otherwise, use strict DeepEqual
	return reflect.DeepEqual(a, b)
}

func isNumeric(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func toFloat64(v interface{}) float64 {
	switch v := v.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

// Hash calculates the hash value of JobParameters. It converts parameters to a canonical JSON string before hashing to ensure order independence.
func (jp JobParameters) Hash() (string, error) {
	// Convert parameters to a canonical JSON string to get a stable hash.
	normalizedJSON, err := jp.toCanonicalJSON()
	if err != nil {
		return "", exception.NewBatchError("job_parameters", "Failed to marshal JobParameters to canonical JSON for hash calculation", err, false, false) // Error during JSON marshaling
	}

	hasher := sha256.New()
	hasher.Write([]byte(normalizedJSON))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// toCanonicalJSON converts JobParameters to a canonical JSON string with sorted keys.
func (jp JobParameters) toCanonicalJSON() (string, error) {
	var marshalCanonical func(interface{}) ([]byte, error)
	marshalCanonical = func(val interface{}) ([]byte, error) {
		if m, ok := val.(map[string]interface{}); ok {
			// Sort map keys for consistent order
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			// Build new map in sorted key order, processing recursively
			var sb strings.Builder // Use strings.Builder for efficient string concatenation
			sb.WriteString("{")
			for i, k := range keys {
				v := m[k]
				keyBytes, err := json.Marshal(k)
				if err != nil {
					return nil, err
				}
				valBytes, err := marshalCanonical(v)
				if err != nil {
					return nil, err
				}
				sb.Write(keyBytes)
				sb.WriteString(":")
				sb.Write(valBytes)
				if i < len(keys)-1 {
					sb.WriteString(",")
				}
			}
			sb.WriteString("}")
			return []byte(sb.String()), nil
		}
		return json.Marshal(val)
	}

	jsonBytes, err := marshalCanonical(jp.Params)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// NewID generates a new UUID string.
func NewID() string {
	return uuid.New().String()
}

// String returns the string representation of JobParameters. Sensitive information is masked.
func (jp JobParameters) String() string {
	maskedParams := serialization.GetMaskedJobParametersMap(jp.Params)

	data, err := json.Marshal(maskedParams)
	if err != nil {
		return fmt.Sprintf("{[ERROR: Failed to marshal masked parameters: %v]}", err)
	}

	return string(data)
}

// NewJobInstance creates a new instance of JobInstance.
func NewJobInstance(jobName string, params JobParameters) *JobInstance {
	now := time.Now()
	hash, err := params.Hash()
	if err != nil {
		logger.Errorf("Failed to calculate JobParameters hash: %v", err)
		hash = "" // Set hash to empty string on error
	}
	return &JobInstance{
		ID:             NewID(),
		JobName:        jobName,
		Parameters:     params,
		CreateTime:     now,
		Version:        0,
		ParametersHash: hash,
	}
}

// NewJobExecution creates a new instance of JobExecution.
func NewJobExecution(jobInstanceID string, jobName string, params JobParameters) *JobExecution {
	now := time.Now()
	return &JobExecution{
		ID:               NewID(),
		JobInstanceID:    jobInstanceID,
		JobName:          jobName,
		Parameters:       params,
		StartTime:        now,
		Status:           BatchStatusStarting,
		ExitStatus:       ExitStatusUnknown,
		CreateTime:       now,
		LastUpdated:      now,
		Failures:         make(FailureList, 0),
		StepExecutions:   make([]*StepExecution, 0),
		ExecutionContext: NewExecutionContext(),
		CurrentStepName:  "",
		CancelFunc:       nil,
		RestartCount:     0,
	}
}

// IncrementRestartCount increments the restart count of JobExecution by 1.
func (je *JobExecution) IncrementRestartCount() {
	je.RestartCount++
	je.LastUpdated = time.Now()
	logger.Debugf("JobExecution (ID: %s) restart count updated to %d.", je.ID, je.RestartCount)
}

// isValidJobTransition checks if the state transition for JobExecution is valid.
func isValidJobTransition(current, next JobStatus) bool {
	switch current { // Evaluate current status
	case BatchStatusStarting:
		// STARTING can transition to STARTED, FAILED, STOPPED, ABANDONED (initial states)
		return next == BatchStatusStarted || next == BatchStatusFailed || next == BatchStatusStopped || next == BatchStatusAbandoned
	case BatchStatusStarted:
		// STARTED can transition to STOPPING, COMPLETED, FAILED, ABANDONED (during execution)
		return next == BatchStatusStopping || next == BatchStatusCompleted || next == BatchStatusFailed || next == BatchStatusAbandoned
	case BatchStatusStopping:
		// STOPPING can transition to STOPPED, STOPPING_FAILED, FAILED, ABANDONED (during shutdown)
		return next == BatchStatusStopped || next == BatchStatusStoppingFailed || next == BatchStatusFailed || next == BatchStatusAbandoned
	case BatchStatusStopped:
		// STOPPED can transition to RESTARTING (only for restart scenarios)
		return next == BatchStatusRestarting
	case BatchStatusFailed:
		// FAILED can transition to ABANDONED (e.g., when a new execution is launched for restart) or RESTARTING
		return next == BatchStatusAbandoned || next == BatchStatusRestarting
	case BatchStatusRestarting:
		return next == BatchStatusStarted || next == BatchStatusFailed || next == BatchStatusStopped || next == BatchStatusAbandoned // After restart attempt
	case BatchStatusCompleted, BatchStatusAbandoned, BatchStatusStoppingFailed:
		return false // Cannot transition directly from terminal states
	default:
		return false
	}
}

// TransitionTo safely transitions the state of JobExecution. Note: Fields other than Status and LastUpdated must be set separately by the caller.
func (je *JobExecution) TransitionTo(newStatus JobStatus) error {
	if !isValidJobTransition(je.Status, newStatus) {
		return fmt.Errorf("JobExecution (ID: %s): Invalid state transition: %s -> %s", je.ID, je.Status, newStatus)
	}
	je.Status = newStatus
	return nil
}

// PartitionName generates a standard partition name from the partition index.
func PartitionName(index int) string {
	return fmt.Sprintf("partition%d", index)
}

// MarkAsStarted updates the JobExecution status to STARTED.
func (je *JobExecution) MarkAsStarted() {
	if err := je.TransitionTo(BatchStatusStarted); err != nil {
		logger.Warnf("Could not update JobExecution (ID: %s) status to STARTED: %v", je.ID, err)
		je.Status = BatchStatusStarted
	}
	je.LastUpdated = time.Now()
}

// MarkAsCompleted updates the JobExecution status to COMPLETED.
func (je *JobExecution) MarkAsCompleted() {
	if err := je.TransitionTo(BatchStatusCompleted); err != nil {
		logger.Warnf("Could not update JobExecution (ID: %s) status to COMPLETED: %v", je.ID, err)
		je.Status = BatchStatusCompleted
	}
	je.ExitStatus = ExitStatusCompleted
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// MarkAsFailed updates the JobExecution status to FAILED and adds error information.
func (je *JobExecution) MarkAsFailed(err error) {
	if err := je.TransitionTo(BatchStatusFailed); err != nil {
		logger.Warnf("Could not update JobExecution (ID: %s) status to FAILED: %v", je.ID, err)
		je.Status = BatchStatusFailed
	}
	je.ExitStatus = ExitStatusFailed
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
	if err != nil {
		je.AddFailureException(err)
	}
}

// MarkAsStopped updates the JobExecution status to STOPPED.
func (je *JobExecution) MarkAsStopped() {
	if err := je.TransitionTo(BatchStatusStopped); err != nil {
		logger.Warnf("Could not update JobExecution (ID: %s) status to STOPPED: %v", je.ID, err)
		je.Status = BatchStatusStopped
	}
	je.ExitStatus = ExitStatusStopped
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// MarkAsAbandoned updates the JobExecution status to ABANDONED.
func (je *JobExecution) MarkAsAbandoned() {
	if err := je.TransitionTo(BatchStatusAbandoned); err != nil {
		logger.Warnf("Could not update JobExecution (ID: %s) status to ABANDONED: %v", je.ID, err)
		je.Status = BatchStatusAbandoned
	}
	je.ExitStatus = ExitStatusAbandoned
	now := time.Now()
	je.EndTime = &now
	je.LastUpdated = now
}

// AddFailureException adds error information to JobExecution. It avoids adding duplicate errors.
func (je *JobExecution) AddFailureException(err error) {
	if err == nil {
		return
	}
	errMsg := exception.ExtractErrorMessage(err)

	for _, existingErr := range je.Failures {
		if existingErr == errMsg { // Check for duplicate error messages
			logger.Debugf("Skipped adding duplicate error '%s' to JobExecution (ID: %s).", errMsg, je.ID)
			return
		}
	}

	je.Failures = append(je.Failures, errMsg)
	je.LastUpdated = time.Now()
}

// AddStepExecution adds a StepExecution to JobExecution.
func (je *JobExecution) AddStepExecution(se *StepExecution) {
	je.StepExecutions = append(je.StepExecutions, se)
}

// NewStepExecution creates a new instance of StepExecution.
func NewStepExecution(id string, jobExecution *JobExecution, stepName string) *StepExecution {
	now := time.Now()
	se := &StepExecution{
		ID:               id,
		StepName:         stepName,
		JobExecutionID:   jobExecution.ID,
		JobExecution:     jobExecution,
		StartTime:        now,
		Status:           BatchStatusStarting,
		ExitStatus:       ExitStatusUnknown,
		Failures:         make(FailureList, 0),
		ExecutionContext: NewExecutionContext(),
		LastUpdated:      now,
		Version:          0,
	}
	return se
}

// isValidStepTransition checks if the state transition for StepExecution is valid.
func isValidStepTransition(current, next JobStatus) bool {
	switch current {
	case BatchStatusStarting: // From initial state
		return next == BatchStatusStarted || next == BatchStatusFailed || next == BatchStatusStopped || next == BatchStatusAbandoned
	case BatchStatusStarted: // During execution
		return next == BatchStatusCompleted || next == BatchStatusFailed || next == BatchStatusStopped || next == BatchStatusAbandoned
	case BatchStatusCompleted, BatchStatusFailed, BatchStatusStopped, BatchStatusAbandoned:
		return false // Cannot transition directly from terminal states
	default:
		return false
	}
}

// TransitionTo safely transitions the state of StepExecution.
func (se *StepExecution) TransitionTo(newStatus JobStatus) error {
	if !isValidStepTransition(se.Status, newStatus) {
		return fmt.Errorf("StepExecution (ID: %s): Invalid state transition: %s -> %s", se.ID, se.Status, newStatus)
	}
	se.Status = newStatus
	return nil
}

// MarkAsStarted updates the StepExecution status to STARTED.
func (se *StepExecution) MarkAsStarted() {
	if err := se.TransitionTo(BatchStatusStarted); err != nil {
		logger.Warnf("Could not update StepExecution (ID: %s) status to STARTED: %v", se.ID, err)
		se.Status = BatchStatusStarted
	}
	se.LastUpdated = time.Now()
}

// MarkAsCompleted updates the StepExecution status to COMPLETED.
func (se *StepExecution) MarkAsCompleted() {
	if err := se.TransitionTo(BatchStatusCompleted); err != nil {
		logger.Warnf("Could not update StepExecution (ID: %s) status to COMPLETED: %v", se.ID, err)
		se.Status = BatchStatusCompleted
	}
	se.ExitStatus = ExitStatusCompleted
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
}

// MarkAsFailed updates the StepExecution status to FAILED and adds error information.
func (se *StepExecution) MarkAsFailed(err error) {
	if err := se.TransitionTo(BatchStatusFailed); err != nil {
		logger.Warnf("Could not update StepExecution (ID: %s) status to FAILED: %v", se.ID, err)
		se.Status = BatchStatusFailed
	}
	se.ExitStatus = ExitStatusFailed
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
	if err != nil {
		se.AddFailureException(err)
	}
}

// MarkAsStopped updates the StepExecution status to STOPPED.
func (se *StepExecution) MarkAsStopped() {
	if err := se.TransitionTo(BatchStatusStopped); err != nil {
		logger.Warnf("Could not update StepExecution (ID: %s) status to STOPPED: %v", se.ID, err)
		se.Status = BatchStatusStopped
	}
	se.ExitStatus = ExitStatusStopped
	now := time.Now()
	se.EndTime = &now
	se.LastUpdated = now
}

// AddFailureException adds error information to StepExecution. It avoids adding duplicate errors.
func (se *StepExecution) AddFailureException(err error) {
	if err == nil {
		return
	}
	errMsg := exception.ExtractErrorMessage(err)

	for _, existingErr := range se.Failures {
		if existingErr == errMsg { // Check for duplicate error messages
			logger.Debugf("Skipped adding duplicate error '%s' to StepExecution (ID: %s).", errMsg, se.ID)
			return
		}
	}

	se.Failures = append(se.Failures, errMsg)
	se.LastUpdated = time.Now()
}
