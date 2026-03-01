// Package logger provides a simple logging utility for the Surfin Batch Framework.
// It wraps the standard `log/slog` package to offer structured logging capabilities
// with configurable levels and formats.
package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// LogLevel is a type representing the logging level.
type LogLevel int

const (
	// LevelDebug is the log level used for detailed debugging information.
	LevelDebug LogLevel = iota
	// LevelInfo is the log level used for general informational messages.
	LevelInfo
	// LevelWarn is the log level used for potential issues or warning messages.
	LevelWarn
	// LevelError is the log level used for error messages.
	LevelError
	// LevelFatal is the log level used for fatal error messages that cause application termination.
	LevelFatal
	// LevelSilent is the log level that suppresses all log output.
	LevelSilent
)

// slogLogger is the global slog logger instance currently in use.
var slogLogger *slog.Logger

// currentLogLevel stores the currently configured global log level.
var currentLogLevel = LevelInfo

// currentLogFormat stores the currently configured global log format ("json" or "text").
var currentLogFormat = "json" // Default is JSON

func init() {
	// init initializes the default slog logger with INFO level and JSON format.
	// JSON format is used by default for compatibility with Cloud Logging.
	updateSlogLogger()
}

// updateSlogLogger reconstructs the global slog logger based on the current log level and format.
func updateSlogLogger() {
	var slogLevel slog.Level
	var handler slog.Handler

	// Map custom LogLevel to slog.Level.
	// LevelFatal is mapped to slog.LevelError as slog does not have a dedicated FATAL level.
	switch currentLogLevel {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError, LevelFatal: // LevelFatal maps to slog.LevelError
		slogLevel = slog.LevelError
	case LevelSilent:
		// For silent, use a handler that discards all logs.
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1}) // Set level higher than any possible log
		slogLogger = slog.New(handler)
		return
	default:
		slogLevel = slog.LevelDebug // Default to DEBUG for diagnosis
	}

	handlerOptions := &slog.HandlerOptions{
		Level: slogLevel,
	}

	// Select log format handler.
	if currentLogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	} else { // "text" or any other value
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	}
	slogLogger = slog.New(handler)
}

// SetLogLevel sets the global log level for the framework.
// Only log messages at or below the specified level will be output to the console.
// Valid string values are "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "SILENT" (case-insensitive).
// If an invalid value is specified, the default "INFO" level is used, and a warning is printed to standard output.
func SetLogLevel(level string) {
	switch strings.ToUpper(level) { // Convert to uppercase for comparison
	case "INFO":
		currentLogLevel = LevelInfo
	case "WARN":
		currentLogLevel = LevelWarn
	case "ERROR":
		currentLogLevel = LevelError
	case "FATAL": // FATAL is handled by os.Exit(1) in Fatalf, but mapped to LevelError for filtering.
		currentLogLevel = LevelFatal
	case "DEBUG":
		currentLogLevel = LevelDebug
	case "SILENT":
		currentLogLevel = LevelSilent
	default:
		fmt.Printf("Unknown log level '%s' specified. Defaulting to INFO level.\n", level) // Log to stderr directly
		currentLogLevel = LevelInfo
	}
	updateSlogLogger() // Update the logger
}

// SetLogFormat sets the global log format for the framework.
// Valid string values are "json" or "text" (case-insensitive).
// If an invalid value is specified, the default "json" format is used.
func SetLogFormat(format string) {
	switch strings.ToLower(format) { // Convert to lowercase for comparison
	case "json":
		currentLogFormat = "json"
	case "text":
		currentLogFormat = "text"
	default:
		fmt.Printf("Unknown log format '%s' specified. Defaulting to JSON format.\n", format)
		currentLogFormat = "json"
	}
	updateSlogLogger() // Update the logger
}

// Debugf formats and outputs a DEBUG level log message.
// It is only output if the current log level is DEBUG or lower.
//
// Parameters:
//
//	format: A format string in the same format as `fmt.Printf`.
//	v: Arguments to pass to the format string.
func Debugf(format string, v ...interface{}) {
	slogLogger.Debug(fmt.Sprintf(format, v...))
}

// Infof formats and outputs an INFO level log message.
// It is only output if the current log level is INFO or lower.
//
// Parameters:
//
//	format: A format string in the same format as `fmt.Printf`.
//	v: Arguments to pass to the format string.
func Infof(format string, v ...interface{}) {
	slogLogger.Info(fmt.Sprintf(format, v...))
}

// Warnf formats and outputs a WARN level log message.
// It is only output if the current log level is WARN or lower.
//
// Parameters:
//
//	format: A format string in the same format as `fmt.Printf`.
//	v: Arguments to pass to the format string.
func Warnf(format string, v ...interface{}) {
	slogLogger.Warn(fmt.Sprintf(format, v...))
}

// Errorf formats and outputs an ERROR level log message.
// It is only output if the current log level is ERROR or lower.
//
// Parameters:
//
//	format: A format string in the same format as `fmt.Printf`.
//	v: Arguments to pass to the format string.
func Errorf(format string, v ...interface{}) {
	slogLogger.Error(fmt.Sprintf(format, v...))
}

// Fatalf formats and outputs a FATAL level log message,
// then terminates the program by calling os.Exit(1).
//
// Parameters:
//
//	format: A format string in the same format as `fmt.Printf`.
//	v: Arguments to pass to the format string.
func Fatalf(format string, v ...interface{}) {
	// Logs at ERROR level and then terminates the program.
	slogLogger.Error(fmt.Sprintf(format, v...), slog.String("level", "FATAL")) // Add "level": "FATAL" attribute for JSON logs
	os.Exit(1)
}
