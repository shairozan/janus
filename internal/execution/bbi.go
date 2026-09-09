package execution

import (
	"context"
	"fmt"

	"github.com/shairozan/janus/internal/config"
)

// BBIExecutor implements BBI-based execution.
type BBIExecutor struct {
	//nolint:unused
	config *config.Config
}

// NewBBIExecutor creates a new BBI executor.
func NewBBIExecutor(cfg *config.Config) Executor {
	return &BBIExecutor{
		config: cfg,
	}
}

// Execute runs a NONMEM model using BBI.
func (e *BBIExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	// TODO: Implement BBI execution
	// This would use the 'bbi' command to execute NONMEM models
	// Example: bbi nonmem run local model.mod --parallel --threads=4

	return nil, fmt.Errorf("BBI execution not yet implemented")
}

// BuildCommand generates the BBI command and arguments for validation testing.
// This method exposes the command generation logic for testing purposes.
func (e *BBIExecutor) BuildCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	return e.buildBBICommand(modelPath, isParallel, cores, isGrid, additionalOptions)
}

// buildBBICommand builds the BBI command with appropriate arguments.
func (e *BBIExecutor) buildBBICommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	// BBI command structure: bbi nonmem run local model.mod [options]
	binary := "bbi"
	args := []string{"nonmem", "run", "local", modelPath}

	// Add parallel execution options
	if isParallel && cores > 1 {
		args = append(args, "--parallel")
		args = append(args, fmt.Sprintf("--threads=%d", cores))
	}

	// Add grid execution options based on scheduler
	if isGrid {
		// Note: BBI SLURM support may be limited - use SGE for all grid execution for now
		args[2] = "sge"
	}

	// Add additional BBI-specific options
	if additionalOptions != nil {
		args = append(args, additionalOptions...)
	}

	return binary, args, nil
}
