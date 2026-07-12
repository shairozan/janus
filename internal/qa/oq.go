package qa

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
	"github.com/pharmalytica/janus/internal/summary"
)

const (
	categoryPreflight  = "preflight"
	categoryFunctional = "functional"

	acopModelName = "acop.mod"
	acopDataName  = "acop.csv"
	acopOutputLst = "acop.lst"
	acopOutputExt = "acop.ext"
)

// ExecutorProvider builds the executor for the OQ functional run, given the path
// of the (already materialized) ACOP model. It is a function rather than a
// pre-built executor because the Hermes executor must be constructed against the
// model path — and RunOQ materializes that model internally. The caller builds
// it from the same factory it uses for real runs, choosing local vs Hermes by
// mode; RunOQ itself never constructs an executor.
type ExecutorProvider func(modelPath string) (execution.Executor, error)

// RunOQ performs the Operational Qualification.
//
// Phase 1 validates that the given configuration is coherent (NONMEM binary,
// license, and mode-specific dependencies). If any required phase-1 check fails
// it records Phase "preflight" and stops. Phase 2 — only reached when phase 1
// passes and the mode supports a synchronous run — materializes the embedded
// ACOP model, builds the executor via provider, runs the model (so it validates
// whatever mode the user configured), and gates on the objective function value
// matching the known reference within tolerance.
//
// provider and summarizer are supplied by the caller via the same factory used
// for real runs; RunOQ never constructs them.
func RunOQ(ctx context.Context, cfg *config.Config, provider ExecutorProvider, summarizer Summarizer, runDir string, opts Options) (*OQResult, error) {
	result := &OQResult{
		Kind:         KindOQ,
		StartedAt:    opts.now(),
		Phase:        PhasePreflight,
		ReferenceOFV: ReferenceOFV,
		AbsTolerance: AbsTolerance,
		RelTolerance: RelTolerance,
	}

	if cfg == nil {
		result.Checks = append(result.Checks, Check{
			Name:     "config_loads",
			Status:   StatusFail,
			Reason:   "configuration failed to load",
			Category: categoryPreflight,
		})
		result.Status = StatusFail
		result.CompletedAt = opts.now()

		return result, nil
	}

	result.JanusVersion = cfg.Version
	result.User = cfg.User

	// ---- Phase 1: configuration validity ----
	result.Checks = append(result.Checks, nonmemBinaryCheck(cfg), nonmemLicenseCheck(cfg))
	result.Checks = append(result.Checks, modeSpecificChecks(cfg)...)

	if overallStatus(result.Checks) == StatusFail {
		result.Status = StatusFail
		result.CompletedAt = opts.now()

		return result, nil
	}

	// Grid (async) functional runs are out of scope for v1; phase-1 has already
	// validated the grid configuration, so record a skip and pass.
	if cfg.Scheduler != "" {
		result.Checks = append(result.Checks, Check{
			Name:     "functional_run",
			Status:   StatusSkip,
			Reason:   "grid (async) functional run is out of scope for v1; phase-1 config validated",
			Category: categoryFunctional,
		})
		result.Status = overallStatus(result.Checks)
		result.CompletedAt = opts.now()

		return result, nil
	}

	// ---- Phase 2: functional ACOP run ----
	result.Phase = PhaseFunctional
	if err := runFunctional(ctx, provider, summarizer, runDir, result); err != nil {
		result.Status = StatusFail
		result.CompletedAt = opts.now()

		return result, err
	}

	result.Status = overallStatus(result.Checks)
	result.CompletedAt = opts.now()

	return result, nil
}

// runFunctional materializes the embedded ACOP model, builds the executor via
// provider, runs it, and appends the phase-2 gate checks. A returned error is an
// infrastructure failure (couldn't materialize the model); check failures —
// including a failed executor build — are recorded on result, not returned.
func runFunctional(ctx context.Context, provider ExecutorProvider, summarizer Summarizer, runDir string, result *OQResult) error {
	modelBytes, err := acopModel()
	if err != nil {
		return fmt.Errorf("loading embedded acop model: %w", err)
	}

	dataBytes, err := acopData()
	if err != nil {
		return fmt.Errorf("loading embedded acop data: %w", err)
	}

	modelPath := filepath.Join(runDir, acopModelName)
	if err := os.WriteFile(modelPath, modelBytes, 0o600); err != nil {
		return fmt.Errorf("writing acop model: %w", err)
	}

	if err := os.WriteFile(filepath.Join(runDir, acopDataName), dataBytes, 0o600); err != nil {
		return fmt.Errorf("writing acop data: %w", err)
	}

	result.Artifacts = append(result.Artifacts, acopModelName, acopDataName)

	// Build the executor now that the model exists (Hermes needs the model path).
	executor, err := provider(modelPath)
	if err != nil {
		result.Checks = append(result.Checks, Check{
			Name:     "executor_build",
			Status:   StatusFail,
			Reason:   fmt.Sprintf("building executor: %v", err),
			Category: categoryFunctional,
		})

		return nil
	}

	// Run through the caller's executor (their configured mode). Working dir and
	// .lst location are derived from modelPath by the executor.
	execResult, execErr := executor.Execute(ctx, modelPath, false, 1, false, nil)

	if execResult != nil {
		ec := execResult.ExitCode
		result.NonmemExitCode = &ec
		result.Artifacts = append(result.Artifacts, persistStreams(runDir, execResult)...)
	}

	for _, name := range []string{acopOutputLst, acopOutputExt} {
		if _, statErr := os.Stat(filepath.Join(runDir, name)); statErr == nil {
			result.Artifacts = append(result.Artifacts, name)
		}
	}

	// Gate A: execution succeeded and produced a .lst.
	lstPath := filepath.Join(runDir, acopOutputLst)
	execCheck := executionCheck(execResult, execErr, lstPath)
	result.Checks = append(result.Checks, execCheck)

	if execCheck.Status != StatusPass {
		return nil
	}

	// Parse the completed run.
	sum, sumErr := summarizer.SummarizeModel(ctx, modelPath, summary.SummaryOptions{})
	if sumErr != nil {
		result.Checks = append(result.Checks, Check{
			Name:     "ofv_present",
			Status:   StatusFail,
			Reason:   fmt.Sprintf("summarizing run: %v", sumErr),
			Category: categoryFunctional,
		})

		return nil
	}

	// Gate B: OFV present and finite.
	ofvCheck := Check{Name: "ofv_present", Category: categoryFunctional}
	ofvPtr := sum.GoodnessOfFit.ObjectiveFunctionValue
	if ofvPtr == nil || math.IsNaN(*ofvPtr) || math.IsInf(*ofvPtr, 0) {
		ofvCheck.Status = StatusFail
		ofvCheck.Reason = "objective function value missing or non-finite"
		result.Checks = append(result.Checks, ofvCheck)

		return nil
	}

	ofv := *ofvPtr
	result.ObservedOFV = &ofv
	ofvCheck.Status = StatusPass
	ofvCheck.Reason = fmt.Sprintf("OFV = %.6f", ofv)
	result.Checks = append(result.Checks, ofvCheck)

	// Gate C: OFV within tolerance of the reference.
	result.Checks = append(result.Checks, toleranceCheck(ofv))

	// Gate D: minimization successful.
	result.Checks = append(result.Checks, minimizationCheck(sum.Estimation))

	return nil
}

// persistStreams writes stdout/stderr to the run dir and returns the artifact
// names that were written.
func persistStreams(runDir string, res *execution.ExecutionResult) []string {
	var written []string

	if err := os.WriteFile(filepath.Join(runDir, "stdout.txt"), res.Stdout, 0o600); err == nil {
		written = append(written, "stdout.txt")
	}

	if err := os.WriteFile(filepath.Join(runDir, "stderr.txt"), res.Stderr, 0o600); err == nil {
		written = append(written, "stderr.txt")
	}

	return written
}

// executionCheck evaluates gate A: no error, exit 0, and a .lst on disk.
func executionCheck(res *execution.ExecutionResult, execErr error, lstPath string) Check {
	c := Check{Name: "nonmem_execution", Category: categoryFunctional}

	_, lstErr := os.Stat(lstPath)

	switch {
	case execErr != nil:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("execution error: %v", execErr)
	case res == nil:
		c.Status = StatusFail
		c.Reason = "no execution result returned"
	case res.ExitCode != 0:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("non-zero exit code %d", res.ExitCode)
	case lstErr != nil:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("output %s not found: %v", filepath.Base(lstPath), lstErr)
	default:
		c.Status = StatusPass
		c.Reason = "exit 0, .lst present"
	}

	return c
}

// toleranceCheck evaluates gate C against ReferenceOFV.
func toleranceCheck(ofv float64) Check {
	c := Check{Name: "ofv_within_tolerance", Category: categoryFunctional}

	if withinTolerance(ofv, ReferenceOFV) {
		c.Status = StatusPass
		c.Reason = fmt.Sprintf("OFV %.6f within tolerance of reference %.6f", ofv, ReferenceOFV)

		return c
	}

	c.Status = StatusFail
	c.Reason = fmt.Sprintf("OFV %.6f outside tolerance of reference %.6f (abs=%g, rel=%g)",
		ofv, ReferenceOFV, AbsTolerance, RelTolerance)

	return c
}

// minimizationCheck evaluates gate D from the parsed estimation summary.
func minimizationCheck(est summary.EstimationSummary) Check {
	c := Check{Name: "minimization_successful", Category: categoryFunctional}

	if est.Minimized {
		c.Status = StatusPass
		c.Reason = "minimization successful"

		return c
	}

	c.Status = StatusFail
	c.Reason = "minimization not successful"
	if est.TerminationReason != "" {
		c.Reason += ": " + est.TerminationReason
	}

	return c
}

// withinTolerance reports whether got is within the absolute OR relative band of
// ref.
func withinTolerance(got, ref float64) bool {
	diff := math.Abs(got - ref)
	if diff <= AbsTolerance {
		return true
	}

	if ref != 0 && diff/math.Abs(ref) <= RelTolerance {
		return true
	}

	return false
}

// nonmemBinaryCheck validates the local NONMEM binary for host-executed modes.
// It is skipped for Hermes (NONMEM lives in the container) and grid (NONMEM runs
// on the compute node).
func nonmemBinaryCheck(cfg *config.Config) Check {
	c := Check{Name: "nonmem_binary", Category: categoryPreflight}

	switch {
	case cfg.ExecutionMode == config.ExecutionModeHERMES:
		c.Status = StatusSkip
		c.Reason = "NONMEM provided by the Hermes container"

		return c
	case cfg.Scheduler != "":
		c.Status = StatusSkip
		c.Reason = "NONMEM runs on the grid compute node"

		return c
	case cfg.NonmemPath == "" || cfg.NonmemBinary == "":
		c.Status = StatusFail
		c.Reason = "nonmem-path and nonmem-binary must be configured"

		return c
	}

	binaryPath, err := filepath.Abs(filepath.Join(cfg.NonmemPath, cfg.NonmemBinary))
	if err != nil {
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("resolving binary path: %v", err)

		return c
	}

	info, err := os.Stat(binaryPath)

	switch {
	case err != nil:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s: %v", binaryPath, err)
	case info.IsDir():
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s is a directory", binaryPath)
	case runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s is not executable", binaryPath)
	default:
		c.Status = StatusPass
		c.Reason = binaryPath
	}

	return c
}

// nonmemLicenseCheck validates license discoverability via the existing fallback
// chain, for modes that require a NONMEM license.
func nonmemLicenseCheck(cfg *config.Config) Check {
	c := Check{Name: "nonmem_license", Category: categoryPreflight}

	if !config.RequiresNONMEMLicense(cfg.ExecutionMode) {
		c.Status = StatusSkip
		c.Reason = "execution mode does not require a NONMEM license"

		return c
	}

	path, err := config.ValidateNONMEMLicenseForExecution(cfg)
	if err != nil {
		c.Status = StatusFail
		c.Reason = err.Error()

		return c
	}

	c.Status = StatusPass
	c.Reason = path

	return c
}

// modeSpecificChecks returns the dependency checks driven by execution mode and
// scheduler.
func modeSpecificChecks(cfg *config.Config) []Check {
	var checks []Check

	if cfg.ExecutionMode == config.ExecutionModeHERMES {
		checks = append(checks, dockerSocketCheck(), hermesImageCheck(cfg))
	}

	if cfg.Scheduler != "" {
		checks = append(checks, gridSubmitCheck(cfg))
	}

	return checks
}

// dockerSocketCheck verifies the docker socket is present for Hermes execution.
func dockerSocketCheck() Check {
	c := Check{Name: "docker_socket", Category: categoryPreflight}

	const sock = "/var/run/docker.sock"

	if _, err := os.Stat(sock); err != nil {
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("docker socket %s not reachable: %v", sock, err)

		return c
	}

	c.Status = StatusPass
	c.Reason = sock

	return c
}

// hermesImageCheck verifies a Hermes container image is configured.
func hermesImageCheck(cfg *config.Config) Check {
	c := Check{Name: "hermes_image", Category: categoryPreflight}

	if cfg.Hermes.Image == "" {
		c.Status = StatusFail
		c.Reason = "hermes.image not configured"

		return c
	}

	c.Status = StatusPass
	c.Reason = cfg.Hermes.Image

	return c
}

// gridSubmitCheck verifies the scheduler's submit binary is on PATH.
func gridSubmitCheck(cfg *config.Config) Check {
	c := Check{Name: "grid_submit_binary", Category: categoryPreflight}

	binary := schedulerSubmitBinary(cfg.Scheduler)
	if binary == "" {
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("unknown scheduler %q", cfg.Scheduler)

		return c
	}

	path, err := exec.LookPath(binary)
	if err != nil {
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s not found on PATH: %v", binary, err)

		return c
	}

	c.Status = StatusPass
	c.Reason = path

	return c
}

// schedulerSubmitBinary maps a scheduler name to its submit command.
func schedulerSubmitBinary(scheduler string) string {
	switch strings.ToUpper(scheduler) {
	case "SLURM":
		return "sbatch"
	case "SGE", "TORQUE", "PBS":
		return "qsub"
	default:
		return ""
	}
}
