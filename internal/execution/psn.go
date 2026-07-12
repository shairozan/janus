package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/remote"
	"github.com/pharmalytica/janus/internal/runlog"
)

// PSNExecutor implements PsN-based execution.
type PSNExecutor struct {
	config    *config.Config
	runLogger *runlog.RunLogger
}

// NewPSNExecutor creates a new PSN executor (no run logging).
func NewPSNExecutor(cfg *config.Config) Executor {
	return &PSNExecutor{
		config: cfg,
	}
}

// NewPSNExecutorWithRunLog creates a PSN executor that records each run (and the
// PsN result artifacts it produces) to the run log when enabled.
func NewPSNExecutorWithRunLog(cfg *config.Config, runLogEnabled bool) Executor {
	e := &PSNExecutor{config: cfg}

	if runLogEnabled {
		runLogPath := cfg.RunLog.Path
		if runLogPath == "" {
			runLogPath = "runlog.jsonl"
		}

		if logger, err := runlog.NewRunLogger(true, runLogPath); err != nil {
			log.Printf("Warning: failed to initialize PSN run logger: %v", err)
		} else {
			e.runLogger = logger
		}
	}

	return e
}

// Execute runs a NONMEM model through PsN's `execute` command. PsN performs any
// grid submission itself (via -slurm/-sge), so this runs the PsN command as a
// local subprocess, capturing output. Optional pre/post R hook scripts run
// around the command, and the run-output policy is applied afterward.
func (e *PSNExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	return e.executeTool(ctx, "execute", modelPath, isParallel, cores, isGrid, additionalOptions)
}

// PSNBuiltinFunctions are the standard PsN analyses surfaced as first-class
// functions in the GUI (in addition to any configured presets).
var PSNBuiltinFunctions = []string{"execute", "vpc", "bootstrap", "scm"}

// resolveFunction maps a PsN analysis name onto the tool to invoke and the args
// to pass: a configured preset takes precedence (its tool + default args),
// otherwise the name must be a known built-in tool. Any additionalOptions are
// appended. This is the single source of truth shared by RunFunction (which
// executes) and BuildFunctionCommand (which only renders), so the previewed
// command always matches what will run.
func (e *PSNExecutor) resolveFunction(name string, additionalOptions []string) (tool string, args []string, err error) {
	if preset, ok := e.psnConfig().Preset(name); ok {
		merged := append(append([]string{}, preset.Args...), additionalOptions...)

		return preset.Tool, merged, nil
	}

	if slices.Contains(PSNBuiltinFunctions, name) {
		return name, additionalOptions, nil
	}

	return "", nil, fmt.Errorf("unknown PsN function %q", name)
}

// RunFunction runs a named PsN analysis (preset or built-in tool).
func (e *PSNExecutor) RunFunction(ctx context.Context, name, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	tool, args, err := e.resolveFunction(name, additionalOptions)
	if err != nil {
		return nil, err
	}

	return e.executeTool(ctx, tool, modelPath, isParallel, cores, isGrid, args)
}

// BuildFunctionCommand renders (without executing) the PsN command for a named
// analysis, mirroring RunFunction. An empty name is the plain execute default.
// Used by the GUI command preview so it reflects the selected analysis.
func (e *PSNExecutor) BuildFunctionCommand(name, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	if name == "" {
		return e.buildToolCommand("execute", modelPath, isParallel, cores, isGrid, additionalOptions)
	}

	tool, args, err := e.resolveFunction(name, additionalOptions)
	if err != nil {
		return "", nil, err
	}

	return e.buildToolCommand(tool, modelPath, isParallel, cores, isGrid, args)
}

// executeTool runs a PsN tool (default "execute" or a preset's tool) with the
// shared pre/post hook + run-policy wrapper, locally or over SSH.
func (e *PSNExecutor) executeTool(ctx context.Context, tool, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not found: %s: %w", modelPath, err)
	}

	if err := e.runHook(ctx, e.psnConfig().PreScript, modelPath); err != nil {
		return nil, fmt.Errorf("PsN pre-run script failed: %w", err)
	}

	// Apply the pre-run output policy (sequential archiving) like the other
	// executors. No-op by default.
	applyBeforeRun(e.config, modelPath)
	applyPreHooks(ctx, e.config, modelPath)

	startTime := time.Now()

	var (
		result *ExecutionResult
		err    error
	)

	if e.config != nil && e.config.Remote.Host != "" {
		result, err = e.runRemote(ctx, tool, modelPath, isParallel, cores, isGrid, additionalOptions)
	} else {
		var binary string
		var args []string

		binary, args, err = e.buildToolCommand(tool, modelPath, isParallel, cores, isGrid, additionalOptions)
		if err != nil {
			return nil, err
		}

		result, err = e.runPSN(ctx, binary, args, filepath.Dir(modelPath))
	}

	if err != nil {
		return nil, err
	}

	// Post-run hook is best-effort: a failed diagnostic must not mask the run.
	if hookErr := e.runHook(ctx, e.psnConfig().PostScript, modelPath); hookErr != nil {
		result.Stderr = append(result.Stderr, []byte("\nPsN post-run script failed: "+hookErr.Error())...)
	}

	applyPostHooks(ctx, e.config, modelPath)
	applyAfterRun(e.config, modelPath)

	e.recordRun(tool, modelPath, result, time.Since(startTime))

	return result, nil
}

// recordRun records a PsN run to the run log, including the result artifacts the
// analysis produced (so they are discoverable / renderable). Best-effort.
func (e *PSNExecutor) recordRun(tool, modelPath string, result *ExecutionResult, duration time.Duration) {
	if e.runLogger == nil || !e.runLogger.IsEnabled() {
		return
	}

	workDir := filepath.Dir(modelPath)

	binary := tool
	if p := e.psnConfig().Path; p != "" {
		binary = filepath.Join(p, tool)
	}

	err := e.runLogger.RecordExecutionWithFiles(
		e.runLogger.GenerateJobID(),
		binary,
		[]string{modelPath},
		string(result.Stdout),
		string(result.Stderr),
		result.ExitCode,
		duration,
		workDir,
		collectPSNArtifacts(workDir, tool),
	)
	if err != nil {
		log.Printf("Warning: failed to record PSN run log for %s: %v", modelPath, err)
	}
}

// psnArtifactGlobs returns the result-file globs to collect for a PsN tool. PsN's
// exact output naming varies by version/options, so collection is best-effort.
func psnArtifactGlobs(tool string) []string {
	switch tool {
	case "bootstrap":
		return []string{"raw_results*.csv", "bootstrap_dir*", "*.lst"}
	case "vpc":
		return []string{"vpc_results.csv", "vpc_dir*", "*.lst"}
	case "scm":
		return []string{"scmlog.txt", "scm_dir*", "*.scm", "*.lst"}
	default: // execute
		return []string{"*.lst", "*.ext", "*.dir", "NM_run*"}
	}
}

// collectPSNArtifacts lists existing files/dirs in workDir matching the tool's
// result globs (deduped, sorted).
func collectPSNArtifacts(workDir, tool string) []string {
	seen := map[string]bool{}

	var out []string

	for _, g := range psnArtifactGlobs(tool) {
		matches, _ := filepath.Glob(filepath.Join(workDir, g))
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true

				out = append(out, m)
			}
		}
	}

	sort.Strings(out)

	return out
}

// BuildCommand generates the PsN execute command for validation testing.
func (e *PSNExecutor) BuildCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	return e.buildToolCommand("execute", modelPath, isParallel, cores, isGrid, additionalOptions)
}

// BuildPresetCommand generates a PsN command for a named preset (e.g. vpc,
// bootstrap, scm). The preset's tool and default args are used, with any
// additionalOptions appended.
func (e *PSNExecutor) BuildPresetCommand(presetName, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	preset, ok := e.psnConfig().Preset(presetName)
	if !ok {
		return "", nil, fmt.Errorf("unknown PsN preset %q", presetName)
	}

	merged := append(append([]string{}, preset.Args...), additionalOptions...)

	return e.buildToolCommand(preset.Tool, modelPath, isParallel, cores, isGrid, merged)
}

// buildToolCommand builds a PsN command for the given tool (defaulting to
// "execute"). The binary resolves to <PSN.Path>/<tool> when a path is
// configured, otherwise the bare tool name (found on PATH).
func (e *PSNExecutor) buildToolCommand(tool, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	if tool == "" {
		tool = "execute"
	}

	binary := tool
	if path := e.psnConfig().Path; path != "" {
		binary = filepath.Join(path, tool)
	}

	args := []string{modelPath}

	if isParallel && cores > 1 {
		args = append(args, fmt.Sprintf("-threads=%d", cores))
	}

	if isGrid {
		if e.config != nil && e.config.Scheduler == "SLURM" {
			args = append(args, "-slurm")
		} else {
			args = append(args, "-sge")
		}
	}

	args = append(args, additionalOptions...)

	return binary, args, nil
}

// runPSN executes the PsN command in workDir, capturing stdout/stderr and the
// exit code.
func (e *PSNExecutor) runPSN(ctx context.Context, binary string, args []string, workDir string) (*ExecutionResult, error) {
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary/args derive from validated config
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute PsN (%s): %w", binary, err)
		}
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

// runHook runs an optional R hook script (via Rscript) in the model's directory,
// passing the model path as an argument. A blank script is a no-op.
func (e *PSNExecutor) runHook(ctx context.Context, script, modelPath string) error {
	if strings.TrimSpace(script) == "" {
		return nil
	}

	cmd := exec.CommandContext(ctx, "Rscript", script, modelPath) //nolint:gosec // script path comes from validated config
	cmd.Dir = filepath.Dir(modelPath)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", script, strings.TrimSpace(string(out)), err)
	}

	return nil
}

// psnConfig returns the PsN configuration, or the zero value when unset.
func (e *PSNExecutor) psnConfig() config.PSNConfig {
	if e.config == nil {
		return config.PSNConfig{}
	}

	return e.config.PSN
}

// runRemote runs the PsN command on the configured remote host over SSH. PsN
// performs any grid submission itself; the model and working-dir paths are
// translated to the remote host, and results are read locally via the shared
// mount (so collection is unchanged).
func (e *PSNExecutor) runRemote(ctx context.Context, tool, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	mapper := remote.NewPathMapper(e.config.Remote.Mounts)

	runner, err := remote.NewRunner(e.config.Remote)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote runner: %w", err)
	}

	remoteModel := mapper.ToRemote(modelPath)

	gridFlag := ""
	if isGrid {
		gridFlag = "-sge"
		if e.config.Scheduler == "SLURM" {
			gridFlag = "-slurm"
		}
	}

	argv := buildRemotePSNArgv(e.psnConfig().Path, tool, remoteModel, isParallel, cores, gridFlag, additionalOptions)

	out, exitCode, err := runner.Run(ctx, path.Dir(remoteModel), argv, "")
	if err != nil {
		return nil, fmt.Errorf("remote PsN execution failed: %w", err)
	}

	return &ExecutionResult{ExitCode: exitCode, Stdout: []byte(out)}, nil
}

// buildRemotePSNArgv builds the PsN command argv for remote execution, using
// forward-slash (remote) paths for the binary and model.
func buildRemotePSNArgv(psnPath, tool, remoteModel string, isParallel bool, cores int, gridFlag string, additionalOptions []string) []string {
	binary := tool
	if psnPath != "" {
		binary = path.Join(psnPath, tool)
	}

	argv := []string{binary, remoteModel}

	if isParallel && cores > 1 {
		argv = append(argv, fmt.Sprintf("-threads=%d", cores))
	}

	if gridFlag != "" {
		argv = append(argv, gridFlag)
	}

	argv = append(argv, additionalOptions...)

	return argv
}
