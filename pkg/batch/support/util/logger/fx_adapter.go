package logger

import (
	"strings"

	"go.uber.org/fx/fxevent"
)

// FxLoggerAdapter is an adapter that implements the fxevent.Logger interface.
type FxLoggerAdapter struct{}

// NewFxLoggerAdapter creates a new instance of FxLoggerAdapter.
func NewFxLoggerAdapter() fxevent.Logger {
	return &FxLoggerAdapter{}
}

// LogEvent logs events from Fx.
func (l *FxLoggerAdapter) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.OnStartExecuting: // Handles OnStartExecuting events.
		Debugf("OnStart hook executing: %s", extractMeaningfulFunctionName(e.FunctionName))
	case *fxevent.OnStartExecuted: // Handles OnStartExecuted events.
		if e.Err != nil {
			Errorf("OnStart hook failed: %s, error: %v", extractMeaningfulFunctionName(e.FunctionName), e.Err)
		} else {
			Debugf("OnStart hook executed: %s", extractMeaningfulFunctionName(e.FunctionName))
		}
	case *fxevent.OnStopExecuting: // Handles OnStopExecuting events.
		Debugf("OnStop hook executing: %s", extractMeaningfulFunctionName(e.FunctionName))
	case *fxevent.OnStopExecuted: // Handles OnStopExecuted events.
		if e.Err != nil {
			Errorf("OnStop hook failed: %s, error: %v", extractMeaningfulFunctionName(e.FunctionName), e.Err)
		} else {
			Debugf("OnStop hook executed: %s", extractMeaningfulFunctionName(e.FunctionName))
		}
	case *fxevent.Supplied:
		if e.Err != nil {
			Errorf("Supplied failed: %v", e.Err)
		} else {
			Debugf("Supplied: %s", e.TypeName)
		}
	case *fxevent.Provided: // Handles Provided events.
		for _, rtype := range e.OutputTypeNames {
			Debugf("Provided: %s", rtype)
		}
		if e.Err != nil {
			Errorf("Provide error: %v", e.Err)
		}
	case *fxevent.Invoking:
		Debugf("Invoking: %s", extractMeaningfulFunctionName(e.FunctionName))
	case *fxevent.Invoked:
		if e.Err != nil {
			Errorf("Invoke failed: %s, error: %v", e.FunctionName, e.Err)
		}
	case *fxevent.Stopping:
		Debugf("Stopping signal received: %s", e.Signal)
	case *fxevent.Stopped:
		if e.Err != nil {
			Errorf("Stop failed, error: %v", e.Err)
		}
	case *fxevent.RollingBack:
		Errorf("Start failed, rolling back, error: %v", e.StartErr)
	case *fxevent.RolledBack:
		if e.Err != nil {
			Errorf("Rollback failed, error: %v", e.Err)
		}
	case *fxevent.Started:
		if e.Err != nil {
			Errorf("Start failed, error: %v", e.Err)
		} else {
			Infof("Application started.")
		}
	case *fxevent.LoggerInitialized:
		if e.Err != nil {
			Errorf("Logger initialization failed, error: %v", e.Err)
		} else {
			Debugf("Custom logger initialized: %s", e.ConstructorName)
		}
	}
}

// extractMeaningfulFunctionName removes anonymous function parts from Fx's FunctionName
// to extract a more meaningful function name (typically the name of a helper function).
func extractMeaningfulFunctionName(funcName string) string {
	// Removes anonymous function suffixes like ".(func1)" or ".func1".
	if idx := strings.LastIndex(funcName, ".func"); idx != -1 {
		// If the part before ".func" is a package name or struct name, use that.
		return funcName[:idx]
	}
	return funcName
}
