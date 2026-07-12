package execution

import (
	"context"
)

// ExecutionResult represents the result of a NONMEM execution.
type ExecutionResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
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
	// ExecuteWithStreaming runs a NONMEM model with real-time output streaming
	// Returns both streaming channels and final result
	// additionalOptions: additional command-line options (used by NONMEM executor)
	ExecuteWithStreaming(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*StreamingOutput, *ExecutionResult, error)
}

// ExecutorFactory creates executors based on execution mode.
type ExecutorFactory interface {
	// CreateExecutor creates an executor for the specified mode
	CreateExecutor(mode string) (Executor, error)
}
