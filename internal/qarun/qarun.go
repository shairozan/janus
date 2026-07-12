// Package qarun wires the dependency-injected internal/qa core to the concrete
// execution stack. It lives outside internal/qa to preserve qa's orthogonality
// (qa never constructs an executor); the binaries and the GUI import qarun to
// obtain a qa.ExecutorProvider and an executor probe built from the same factory
// used for real runs.
package qarun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
	"github.com/pharmalytica/janus/internal/qa"
)

// defaultHermesMemory is used when the global config does not specify one.
const defaultHermesMemory = "4Gi"

// BuildExecutorProvider returns a qa.ExecutorProvider that builds the executor
// for the OQ functional run using the same factory used for real runs. For
// Hermes it first materializes a .janus.config.json next to the (already
// written) model so CreateHermesExecutor can find it.
func BuildExecutorProvider(cfg *config.Config) qa.ExecutorProvider {
	return func(modelPath string) (execution.Executor, error) {
		factory := execution.NewExecutorFactory(cfg)

		if cfg.ExecutionMode == config.ExecutionModeHERMES {
			if err := writeHermesModelConfig(cfg, modelPath); err != nil {
				return nil, fmt.Errorf("generating .janus.config.json: %w", err)
			}

			return factory.CreateHermesExecutor(modelPath)
		}

		return factory.CreateExecutor(cfg.ExecutionMode)
	}
}

// writeHermesModelConfig derives a .janus.config.json from the global Hermes
// configuration and writes it next to modelPath.
func writeHermesModelConfig(cfg *config.Config, modelPath string) error {
	cores := 1
	if raw := strings.TrimSpace(cfg.Hermes.Resources.CPUs); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cores = n
		}
	}

	memory := strings.TrimSpace(cfg.Hermes.Resources.Memory)
	if memory == "" {
		memory = defaultHermesMemory
	}

	modelCfg := &config.ModelConfig{
		Hermes: &config.HermesExecutionConfig{
			Image: cfg.Hermes.Image,
			Resources: config.ResourceConfig{
				CPUCores: cores,
				Memory:   memory,
			},
		},
	}

	return config.SaveModelConfig(modelPath, modelCfg)
}

// DefaultQABaseDir returns the per-user QA base directory
// (~/.config/janus/qa), falling back to ./qa when the home directory cannot be
// determined. It mirrors the config-dir logic used elsewhere in Janus.
func DefaultQABaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "qa"
	}

	return filepath.Join(home, ".config", "janus", "qa")
}

// NewExecutorProbe returns a qa.Options.ExecutorProbe that runs the bundled
// executor binary (located next to the current process) with --executor-version
// and returns its first output line. It is used by IQ's executor_component
// check.
func NewExecutorProbe() func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		path, err := LocateExecutor()
		if err != nil {
			return "", err
		}

		out, err := exec.CommandContext(ctx, path, "--executor-version").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("running %s --executor-version: %w", filepath.Base(path), err)
		}

		version := strings.TrimSpace(string(out))
		if idx := strings.IndexByte(version, '\n'); idx >= 0 {
			version = version[:idx]
		}

		return version, nil
	}
}

// LocateExecutor returns the path to the executor binary that ships alongside
// the current process (janus and executor are installed side by side).
func LocateExecutor() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating current executable: %w", err)
	}

	name := "executor"
	if runtime.GOOS == "windows" {
		name = "executor.exe"
	}

	candidate := filepath.Join(filepath.Dir(self), name)
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("executor binary not found at %s: %w", candidate, err)
	}

	return candidate, nil
}
