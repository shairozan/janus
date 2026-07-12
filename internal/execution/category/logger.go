package category

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// log is the package-level logger for category operations.
// It is configured once during package initialization and shared across
// all category implementations.
var log = logrus.New()

//nolint:gochecknoinits // Package-level logger configuration; tracked in issue #106 for refactoring
func init() {
	// Configure logger output to stderr (standard for application logs)
	log.SetOutput(os.Stderr)

	// Set log level based on EXECUTOR_DEBUG environment variable
	// EXECUTOR_DEBUG=true or EXECUTOR_DEBUG=1 enables debug logging
	// This allows detailed execution tracing for troubleshooting
	debugEnv := strings.ToLower(os.Getenv("EXECUTOR_DEBUG"))
	if debugEnv == "true" || debugEnv == "1" {
		log.SetLevel(logrus.DebugLevel)
		log.Debug("Executor debug logging enabled")
	} else {
		// Default to Warn level for normal operation
		// This shows only warnings and errors, keeping output clean
		log.SetLevel(logrus.WarnLevel)
	}

	// Use a consistent format without timestamps or colors
	// This keeps logs clean and parseable in containerized environments
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		ForceColors:      false,
		DisableColors:    true,
		PadLevelText:     true,
	})
}

// GetLogger returns the package-level logger for testing purposes.
// This allows tests to verify logging behavior without modifying global state.
func GetLogger() *logrus.Logger {
	return log
}
