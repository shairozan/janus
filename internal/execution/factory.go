package execution

import (
	"fmt"

	"github.com/pharmalytica/janus/internal/config"
)

// DefaultExecutorFactory implements ExecutorFactory.
type DefaultExecutorFactory struct {
	config       *config.Config
	auditEnabled bool
}

// NewExecutorFactory creates a new executor factory.
func NewExecutorFactory(cfg *config.Config) ExecutorFactory {
	return &DefaultExecutorFactory{
		config:       cfg,
		auditEnabled: false, // Default to disabled - set via SetAuditEnabled
	}
}

// SetAuditEnabled sets whether audit logging is enabled for executors.
func (f *DefaultExecutorFactory) SetAuditEnabled(enabled bool) {
	f.auditEnabled = enabled
}

// CreateExecutor creates an executor based on the execution mode.
func (f *DefaultExecutorFactory) CreateExecutor(mode string) (Executor, error) {
	switch mode {
	case config.ExecutionModeNONMEM:
		return NewNONMEMExecutorWithAudit(f.config, f.auditEnabled), nil

	case config.ExecutionModeBBI:
		return NewBBIExecutor(f.config), nil

	case config.ExecutionModePSN:
		return NewPSNExecutor(f.config), nil

	default:
		return nil, fmt.Errorf("unsupported execution mode: %s", mode)
	}
}
