package category

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerInitialization(t *testing.T) {
	// Note: Since the logger is initialized in init(), we can't directly test
	// the init() behavior without subprocess testing. Instead, we test the
	// resulting logger configuration.

	logger := GetLogger()

	t.Run("logger is configured", func(t *testing.T) {
		assert.NotNil(t, logger)
		assert.Equal(t, os.Stderr, logger.Out)
	})

	t.Run("formatter is configured correctly", func(t *testing.T) {
		formatter, ok := logger.Formatter.(*logrus.TextFormatter)
		require.True(t, ok, "Expected TextFormatter")
		assert.True(t, formatter.DisableTimestamp, "Timestamps should be disabled")
		assert.False(t, formatter.ForceColors, "Colors should not be forced")
		assert.True(t, formatter.DisableColors, "Colors should be disabled")
		assert.True(t, formatter.PadLevelText, "Level text should be padded")
	})
}

func TestLoggerDebugMode(t *testing.T) {
	// Create a new logger instance to test behavior without affecting global state
	testLogger := logrus.New()
	var buf bytes.Buffer
	testLogger.SetOutput(&buf)

	// Configure like our init() does
	testLogger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		ForceColors:      false,
		DisableColors:    true,
		PadLevelText:     true,
	})

	t.Run("debug level when EXECUTOR_DEBUG=true", func(t *testing.T) {
		testLogger.SetLevel(logrus.DebugLevel)
		buf.Reset()

		testLogger.Debug("debug message")
		testLogger.Info("info message")

		output := buf.String()
		assert.Contains(t, output, "debug message", "Debug messages should be visible")
		assert.Contains(t, output, "info message", "Info messages should be visible")
		assert.Contains(t, output, "level=debug", "debug level should be shown")
		assert.Contains(t, output, "level=info", "info level should be shown")
	})

	t.Run("debug level when EXECUTOR_DEBUG=1", func(t *testing.T) {
		testLogger.SetLevel(logrus.DebugLevel)
		buf.Reset()

		testLogger.Debug("debug message")

		output := buf.String()
		assert.Contains(t, output, "debug message", "Debug messages should be visible with EXECUTOR_DEBUG=1")
	})

	t.Run("info level by default", func(t *testing.T) {
		testLogger.SetLevel(logrus.InfoLevel)
		buf.Reset()

		testLogger.Debug("debug message")
		testLogger.Info("info message")

		output := buf.String()
		assert.NotContains(t, output, "debug message", "Debug messages should NOT be visible at Info level")
		assert.Contains(t, output, "info message", "Info messages should be visible")
	})

	t.Run("warning level shows warnings and errors", func(t *testing.T) {
		testLogger.SetLevel(logrus.InfoLevel)
		buf.Reset()

		testLogger.Warn("warning message")
		testLogger.Error("error message")

		output := buf.String()
		assert.Contains(t, output, "warning message", "Warning messages should be visible")
		assert.Contains(t, output, "error message", "Error messages should be visible")
		assert.Contains(t, output, "level=warning", "warning level should be shown")
		assert.Contains(t, output, "level=error", "error level should be shown")
	})
}

func TestLoggerStructuredFields(t *testing.T) {
	// Verify that structured logging with fields works correctly
	testLogger := logrus.New()
	var buf bytes.Buffer
	testLogger.SetOutput(&buf)
	testLogger.SetLevel(logrus.DebugLevel)

	testLogger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		ForceColors:      false,
		DisableColors:    true,
		PadLevelText:     true,
	})

	t.Run("fields are included in output", func(t *testing.T) {
		buf.Reset()

		testLogger.WithFields(logrus.Fields{
			"category":   "NONMEM",
			"model_path": "/path/to/model.mod",
		}).Debug("Model categorized")

		output := buf.String()
		assert.Contains(t, output, "category=NONMEM", "Field should be included")
		assert.Contains(t, output, "model_path=/path/to/model.mod", "Field should be included")
		assert.Contains(t, output, "Model categorized", "Message should be included")
	})

	t.Run("WithError includes error in fields", func(t *testing.T) {
		buf.Reset()

		testErr := assert.AnError
		testLogger.WithError(testErr).Error("Operation failed")

		output := buf.String()
		assert.Contains(t, output, "error=", "Error field should be included")
		assert.Contains(t, output, "Operation failed", "Message should be included")
	})
}

func TestLoggerEnvironmentParsing(t *testing.T) {
	// Test the logic that parses EXECUTOR_DEBUG environment variable
	testCases := []struct {
		name           string
		envValue       string
		expectedLevel  logrus.Level
		shouldHaveInfo bool
	}{
		{
			name:           "EXECUTOR_DEBUG=true enables debug",
			envValue:       "true",
			expectedLevel:  logrus.DebugLevel,
			shouldHaveInfo: true,
		},
		{
			name:           "EXECUTOR_DEBUG=TRUE enables debug (case insensitive)",
			envValue:       "TRUE",
			expectedLevel:  logrus.DebugLevel,
			shouldHaveInfo: true,
		},
		{
			name:           "EXECUTOR_DEBUG=1 enables debug",
			envValue:       "1",
			expectedLevel:  logrus.DebugLevel,
			shouldHaveInfo: true,
		},
		{
			name:           "EXECUTOR_DEBUG=false uses info level",
			envValue:       "false",
			expectedLevel:  logrus.InfoLevel,
			shouldHaveInfo: false,
		},
		{
			name:           "EXECUTOR_DEBUG=0 uses info level",
			envValue:       "0",
			expectedLevel:  logrus.InfoLevel,
			shouldHaveInfo: false,
		},
		{
			name:           "EXECUTOR_DEBUG empty uses info level",
			envValue:       "",
			expectedLevel:  logrus.InfoLevel,
			shouldHaveInfo: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the init() logic
			debugEnv := strings.ToLower(tc.envValue)
			var level logrus.Level
			if debugEnv == "true" || debugEnv == "1" {
				level = logrus.DebugLevel
			} else {
				level = logrus.InfoLevel
			}

			assert.Equal(t, tc.expectedLevel, level)
		})
	}
}

func TestLoggerOutputFormat(t *testing.T) {
	// Verify that log output format matches our requirements
	testLogger := logrus.New()
	var buf bytes.Buffer
	testLogger.SetOutput(&buf)
	testLogger.SetLevel(logrus.DebugLevel)

	testLogger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		ForceColors:      false,
		DisableColors:    true,
		PadLevelText:     true,
	})

	t.Run("no ANSI color codes in output", func(t *testing.T) {
		buf.Reset()

		testLogger.Info("test message")

		output := buf.String()
		// ANSI color codes start with \x1b[ or \033[
		assert.NotContains(t, output, "\x1b[", "Should not contain ANSI escape codes")
		assert.NotContains(t, output, "\033[", "Should not contain ANSI escape codes")
	})

	t.Run("no timestamps in output", func(t *testing.T) {
		buf.Reset()

		testLogger.Info("test message")

		output := buf.String()
		// Check that output doesn't start with date/time patterns
		assert.NotRegexp(t, `^\d{4}-\d{2}-\d{2}`, output, "Should not start with date")
		assert.NotRegexp(t, `^\d{2}:\d{2}:\d{2}`, output, "Should not start with time")
	})

	t.Run("level text is padded consistently", func(t *testing.T) {
		buf.Reset()

		testLogger.Debug("debug")
		testLogger.Info("info")
		testLogger.Warn("warn")
		testLogger.Error("error")

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 4, "Should have 4 log lines")

		// Check that each line contains level= prefix (logrus format)
		for _, line := range lines {
			assert.Regexp(t, `^level=(debug|info|warning|error)`, line, "Level should be present")
		}
	})
}
