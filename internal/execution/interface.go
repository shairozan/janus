package execution

import (
	"context"
	"io"

	"github.com/pharmalytica/janus/internal/runlog"
)

// ExecutionResult represents the result of a NONMEM execution.
type ExecutionResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte

	// Container provenance for Hermes executions (CFR 21 Part 11 compliance).
	// Nil for non-containerized executions.
	Container *runlog.ContainerProvenance
}

// StreamingOutput represents real-time output channels.
type StreamingOutput struct {
	Stdout chan string // Channel for stdout lines
	Stderr chan string // Channel for stderr lines
	Done   chan bool   // Channel to signal completion
}

// Executor defines the interface for NONMEM execution implementations.
type Executor interface {
	// Execute runs a NONMEM model and returns the result
	// ctx: context for cancellation and timeouts
	// modelPath: absolute path to the NONMEM model file
	// isParallel: whether to run in parallel mode
	// cores: number of cores to use (only relevant if isParallel is true)
	// isGrid: whether to submit to grid scheduler
	// additionalOptions: additional command-line options (used by NONMEM executor)
	Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error)
}

// StreamingExecutor extends Executor with real-time output streaming capabilities.
type StreamingExecutor interface {
	Executor
	// SetOutputWriters configures optional writers for real-time output streaming.
	// If set, stdout and stderr will be written to these writers as they arrive.
	SetOutputWriters(stdout, stderr io.Writer)
}

// ExecutorFactory creates executors based on execution mode.
type ExecutorFactory interface {
	// CreateExecutor creates an executor for the specified mode
	CreateExecutor(mode string) (Executor, error)

	// CreateHermesExecutor creates a Hermes executor with model-specific configuration.
	// This is separate from CreateExecutor because Hermes REQUIRES a .janus.config.json
	// file co-located with the model. The modelPath is used to locate and load this config.
	//
	// Returns an error if:
	// - .janus.config.json doesn't exist
	// - Config file is malformed
	// - Config validation fails
	CreateHermesExecutor(modelPath string) (Executor, error)
}
