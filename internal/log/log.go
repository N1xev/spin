// Package log provides a project-level charmbracelet/log logger for spin.
//
// It is intended for diagnostic, warning, and error messages — not for
// styled user-facing success/info output, which remains in cmd/print.go.
// The default logger writes to stderr with colors enabled, no timestamps,
// and the info level.
package log

import (
	"os"

	"charm.land/log/v2"
)

// Logger is the package-level logger used by spin. It writes to stderr
// and is intended for warnings, errors, and diagnostic output. Callers
// should prefer the Info/Warn/Error/Debug/Print helpers below.
var Logger = log.NewWithOptions(os.Stderr, log.Options{
	Level:           log.InfoLevel,
	ReportTimestamp: false,
})

// Stdout is a secondary logger for user-facing informational output that
// should appear on stdout (e.g. success messages). It shares the same
// timestamp/level settings as Logger.
var Stdout = log.NewWithOptions(os.Stdout, log.Options{
	Level:           log.InfoLevel,
	ReportTimestamp: false,
})

// Info logs an info-level message to stderr.
func Info(msg string, args ...any) { Logger.Info(msg, args...) }

// Warn logs a warning-level message to stderr.
func Warn(msg string, args ...any) { Logger.Warn(msg, args...) }

// Error logs an error-level message to stderr.
func Error(msg string, args ...any) { Logger.Error(msg, args...) }

// Debug logs a debug-level message to stderr.
func Debug(msg string, args ...any) { Logger.Debug(msg, args...) }

// Print logs a message without a level prefix to stderr.
func Print(msg string, args ...any) { Logger.Print(msg, args...) }

// Fatal logs an error-level message to stderr and exits.
func Fatal(msg string, args ...any) { Logger.Fatal(msg, args...) }

// SetLevel changes the current logging level for both loggers.
func SetLevel(l log.Level) {
	Logger.SetLevel(l)
	Stdout.SetLevel(l)
}
