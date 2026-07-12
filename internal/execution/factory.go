package execution

import (
	"fmt"

	"github.com/pharmalytica/janus/internal/config"
)

// DefaultExecutorFactory implements ExecutorFactory.
type DefaultExecutorFactory struct {
	config        *config.Config
	runLogEnabled bool
}

// NewExecutorFactory creates a new executor factory.
func NewExecutorFactory(cfg *config.Config) ExecutorFactory {
	return &DefaultExecutorFactory{
		config:        cfg,
		runLogEnabled: false, // Default to disabled - set via SetRunLogEnabled
	}
}

// SetRunLogEnabled sets whether run log is enabled for executors.
func (f *DefaultExecutorFactory) SetRunLogEnabled(enabled bool) {
	f.runLogEnabled = enabled
}

// CreateExecutor creates an executor based on the execution mode.
func (f *DefaultExecutorFactory) CreateExecutor(mode string) (Executor, error) {
	switch mode {
	case config.ExecutionModeNONMEM:
		return NewNONMEMExecutorWithRunLog(f.config, f.runLogEnabled), nil

	case config.ExecutionModeBBI:
		return NewBBIExecutor(f.config), nil

	case config.ExecutionModePSN:
		return NewPSNExecutorWithRunLog(f.config, f.runLogEnabled), nil

	case config.ExecutionModeHERMES:
		// Hermes execution requires model-specific config
		// Callers should use CreateHermesExecutor() instead
		return nil, fmt.Errorf("hermes execution requires model-specific configuration - use CreateHermesExecutor(modelPath) instead")

	default:
		return nil, fmt.Errorf("unsupported execution mode: %s", mode)
	}
}

// CreateHermesExecutor creates a Hermes executor with model-specific configuration.
// This loads .janus.config.json from the model's directory and uses it to configure
// the container image and resource requirements.
//
// The .janus.config.json file MUST exist - if it doesn't, this returns an error
// prompting the user to create it.
func (f *DefaultExecutorFactory) CreateHermesExecutor(modelPath string) (Executor, error) {
	// Load the full model config (retain is a top-level, model-wide property, so
	// LoadHermesModelConfig — which only returns the hermes section — is not enough).
	modelCfg, err := config.LoadModelConfig(modelPath)
	if err != nil {
		return nil, err
	}

	if modelCfg.Hermes == nil {
		return nil, fmt.Errorf("hermes execution requires .janus.config.json with a hermes section: %s", config.ConfigPath(modelPath))
	}

	if err := modelCfg.Hermes.Validate(); err != nil {
		return nil, err
	}

	// Create the Hermes executor and wire the model's retain (per-model tier).
	executor := NewHermesExecutor(f.config, modelCfg.Hermes)
	executor.SetModelRetain(modelCfg.Retain)

	return executor, nil
}

// CreateBootstrapSaga builds the host-orchestrated horizontal PsN-bootstrap saga
// for the model on Kubernetes (#192). It loads .janus.config.json for the
// execution image + resources and resolves the PsN orchestration image. Use it
// when IsBootstrapSagaRun reports the saga axes are selected.
func (f *DefaultExecutorFactory) CreateBootstrapSaga(modelPath string) (*BootstrapSaga, error) {
	modelConfig, err := config.LoadHermesModelConfig(modelPath)
	if err != nil {
		return nil, err
	}

	return NewBootstrapSaga(f.config, modelConfig)
}
