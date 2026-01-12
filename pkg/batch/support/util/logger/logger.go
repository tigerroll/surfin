// Package logger provides a simple logging utility for the Surfin Batch Framework.
// It wraps the standard `log` package and filters and outputs messages based on log levels.
package logger

import (
	"fmt"
	"log"
	"strings"
)

// LogLevel is a type representing the logging level.
type LogLevel int

const (
	// LevelDebug is the log level used for detailed debugging information.
	// Smaller numbers indicate more detailed log levels.
	LevelDebug LogLevel = iota
	// LevelInfo is the log level used for general informational messages.
	// Smaller numbers indicate more detailed log levels.
	LevelInfo
	// LevelWarn is the log level used for potential issues or warning messages.
	// Smaller numbers indicate more detailed log levels.
	LevelWarn
	// LevelError is the log level used for error messages.
	// Smaller numbers indicate more detailed log levels.
	LevelError
	// LevelFatal is the log level used for fatal error messages that cause application termination.
	// Smaller numbers indicate more detailed log levels.
	LevelFatal
)

// logLevel is the currently set global log level. Only messages at or below this level will be output.
var logLevel = LevelInfo

// SetLogLevel sets the global log level for the framework.
// Only log messages at or below the specified level will be output to the console.
// Valid string values are "DEBUG", "INFO", "WARN", "ERROR", "FATAL" (case-insensitive).
// If an invalid value is specified, the default "INFO" level is used, and a warning is printed to standard output.
func SetLogLevel(level string) {
	switch strings.ToUpper(level) {
	case "INFO":
		logLevel = LevelInfo
	case "WARN":
		logLevel = LevelWarn
	case "ERROR":
		logLevel = LevelError
	case "FATAL":
		logLevel = LevelFatal
	case "DEBUG": // Moved to the end to ensure default is INFO if not matched.
		logLevel = LevelDebug
	default:
		fmt.Printf("Unknown log level '%s' specified. Defaulting to INFO level.\n", level) // Log to stderr directly.
		logLevel = LevelInfo
	}
}

// Debugf formats and outputs a DEBUG level log message.
// It is only output if the current log level is DEBUG or lower.
//
// format: A format string in the same format as `fmt.Printf`.
// v: Arguments to pass to the format string.
func Debugf(format string, v ...interface{}) {
	if logLevel <= LevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Infof formats and outputs an INFO level log message.
// It is only output if the current log level is INFO or lower.
//
// format: A format string in the same format as `fmt.Printf`.
// v: Arguments to pass to the format string.
func Infof(format string, v ...interface{}) {
	if logLevel <= LevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warnf formats and outputs a WARN level log message.
// It is only output if the current log level is WARN or lower.
//
// format: A format string in the same format as `fmt.Printf`.
// v: Arguments to pass to the format string.
func Warnf(format string, v ...interface{}) {
	if logLevel <= LevelWarn {
		log.Printf("[WARN] "+format, v...)
	}
}

// Errorf formats and outputs an ERROR level log message.
// It is only output if the current log level is ERROR or lower.
//
// format: A format string in the same format as `fmt.Printf`.
// v: Arguments to pass to the format string.
func Errorf(format string, v ...interface{}) {
	if logLevel <= LevelError {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Fatalf formats and outputs a FATAL level log message,
// then terminates the program by calling os.Exit(1).
//
// format: A format string in the same format as `fmt.Printf`.
// v: Arguments to pass to the format string.
func Fatalf(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
