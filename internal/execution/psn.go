package execution

import (
	"context"
	"fmt"

	"github.com/pharmalytica/janus/internal/config"
)

// PSNExecutor implements PsN-based execution.
type PSNExecutor struct {
	//nolint:unused
	config *config.Config
}

// NewPSNExecutor creates a new PSN executor.
func NewPSNExecutor(cfg *config.Config) Executor {
	return &PSNExecutor{
		config: cfg,
	}
}

// Execute runs a NONMEM model using PsN's execute command.
func (e *PSNExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	// TODO: Implement PSN execution
	// This would use the 'execute' command from PsN toolkit
	// Example: execute model.mod -threads=4

	return nil, fmt.Errorf("PSN execution not yet implemented")
}

// BuildCommand generates the PSN execute command and arguments for validation testing.
// This method exposes the command generation logic for testing purposes.
func (e *PSNExecutor) BuildCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	return e.buildPSNCommand(modelPath, isParallel, cores, isGrid, additionalOptions)
}

// buildPSNCommand builds the PSN execute command with appropriate arguments.
func (e *PSNExecutor) buildPSNCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	// PSN execute command structure: execute model.mod [options]
	binary := "execute"
	args := []string{modelPath}

	// Add parallel execution options
	if isParallel && cores > 1 {
		args = append(args, fmt.Sprintf("-threads=%d", cores))
	}

	// Add grid execution options based on scheduler
	if isGrid {
		// Check if we have config and what scheduler is configured
		if e.config != nil && e.config.Scheduler == "SLURM" {
			args = append(args, "-slurm")
		} else {
			// Default to SGE for backward compatibility
			args = append(args, "-sge")
		}
	}

	// Add additional PSN-specific options
	if additionalOptions != nil {
		args = append(args, additionalOptions...)
	}

	return binary, args, nil
}
