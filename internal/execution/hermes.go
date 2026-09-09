package execution

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution/category"
	"github.com/shairozan/janus/internal/runlog"
)

// HermesExecutor implements execution via Hermes gRPC container proxy.
// This executor handles local container execution only (no grid scheduler integration yet).
type HermesExecutor struct {
	config        *config.Config
	modelConfig   *config.HermesExecutionConfig
	runLogger     *runlog.RunLogger
	dockerClient  *client.Client
	stdoutWriter  io.Writer              // Optional writer for real-time stdout streaming
	stderrWriter  io.Writer              // Optional writer for real-time stderr streaming
	detector      *category.Detector     // Model category detector
	modelCategory category.ModelCategory // Category for current model being executed

	// PsN-tool mode: when psnFunction is set (e.g. "vpc", "scm"), Execute runs that
	// PsN tool in the container instead of NONMEM directly, and retains the PsN
	// run directory. Set via SetPSNFunction; empty means a plain NONMEM run.
	psnFunction string
	psnArgs     []string

	// retain is the model's own retain list (top-level ModelConfig.Retain), set via
	// SetModelRetain. It is the per-model tier in resolveRetainPatterns. Empty means
	// inherit the global/category default.
	retain []string
}

// SetPSNFunction puts the executor in PsN-tool mode: Execute will run the named
// PsN tool (vpc/scm/…) against the model with these args, instead of running
// NONMEM directly. The container image must provide PsN (and NONMEM). Passing an
// empty function restores the plain NONMEM behaviour.
func (e *HermesExecutor) SetPSNFunction(function string, args []string) {
	e.psnFunction = function
	e.psnArgs = args
}

// SetModelRetain sets the model's own retain globs (from top-level
// ModelConfig.Retain) — the per-model tier used by resolveRetainPatterns. The
// factory wires this because retain is a model-wide property that no longer lives
// on the HermesExecutionConfig the executor is constructed with.
func (e *HermesExecutor) SetModelRetain(retain []string) {
	e.retain = retain
}

// NewHermesExecutor creates a new Hermes executor with model-specific configuration.
// The modelConfig parameter is required and must not be nil - it specifies the container
// image and resources to use for this specific model.
func NewHermesExecutor(cfg *config.Config, modelConfig *config.HermesExecutionConfig) *HermesExecutor {
	if modelConfig == nil {
		panic("HermesExecutor requires model config - cannot be nil")
	}

	executor := &HermesExecutor{
		config:      cfg,
		modelConfig: modelConfig,
		detector:    category.NewDetector(), // Initialize model category detector
	}

	// Initialize the Docker client — but only for the Docker orchestrator. On
	// Kubernetes the executor drives pods via the kube orchestrator (executeOnKubernetes)
	// and never touches Docker, so constructing a client there is both wasteful and
	// misleading (it logged "Using configured Docker socket …" on a pure-K8s run).
	// The Docker code paths all nil-check e.dockerClient, so leaving it nil is safe.
	if cfg == nil || cfg.Orchestrator != config.OrchestratorKubernetes {
		// If docker_socket is configured, use it explicitly; otherwise use FromEnv.
		var dockerOpts []client.Opt
		if cfg != nil && cfg.Hermes.Container.DockerSocket != "" {
			log.Printf("Using configured Docker socket: %s", cfg.Hermes.Container.DockerSocket)
			dockerOpts = append(dockerOpts, client.WithHost(cfg.Hermes.Container.DockerSocket))
		} else {
			// Use FromEnv which respects DOCKER_HOST, DOCKER_CONTEXT, etc.
			dockerOpts = append(dockerOpts, client.FromEnv)
		}
		dockerOpts = append(dockerOpts, client.WithAPIVersionNegotiation())

		cli, err := client.NewClientWithOpts(dockerOpts...)
		if err != nil {
			log.Printf("Warning: Failed to create Docker client: %v", err)
			log.Printf("Hint: Ensure Docker daemon is running. For Docker Desktop, set hermes.container.docker_socket in config")
			// Continue without Docker client - will fail at execution time with better error
		} else {
			executor.dockerClient = cli
		}
	}

	// Initialize run logger if run log is enabled
	if cfg != nil && cfg.RunLogEnabled {
		auditLogPath := cfg.RunLog.Path
		if auditLogPath == "" {
			auditLogPath = "runlog.jsonl" // Default audit log file
		}

		runLogger, err := runlog.NewRunLogger(true, auditLogPath)
		if err != nil {
			// Log error but don't fail - audit logging is supplementary
			log.Printf("Warning: Failed to initialize run logger: %v", err)
		} else {
			executor.runLogger = runLogger
		}
	}

	return executor
}

// SetOutputWriters configures optional writers for real-time output streaming.
// If set, stdout and stderr will be written to these writers as they arrive,
// in addition to being buffered in the ExecutionResult.
// Pass nil for either writer to disable streaming for that output.
func (e *HermesExecutor) SetOutputWriters(stdout, stderr io.Writer) {
	e.stdoutWriter = stdout
	e.stderrWriter = stderr
}

// Execute runs a NONMEM model via Hermes container execution.
// This implementation handles:
// 1. Creating a Hermes container
// 2. Waiting for it to start and become healthy
// 3. Reading the license file
// 4. Collecting model files
// 5. Making gRPC execution request
// 6. Streaming results back
// 7. Cleaning up the container
// 8. Logging to audit trail with full Hermes metadata
//
// NOTE: This is for LOCAL execution only. Grid scheduler integration is not yet implemented.
func (e *HermesExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	// Grid execution not supported (and never will be - Hermes is container-only)
	if isGrid {
		return nil, fmt.Errorf("hermes execution mode does not support grid execution - it runs in local containers only")
	}

	// Step 0: Detect model category FIRST (before any other setup)
	categoryType, err := e.detector.Detect(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect model category: %w", err)
	}

	category.GetLogger().WithFields(map[string]interface{}{
		"model_path": modelPath,
		"category":   categoryType,
	}).Debug("Model categorized")

	// Create category-specific handler based on detection result
	switch categoryType {
	case category.CategoryNONMEM:
		e.modelCategory = category.NewNONMEMCategory()
		category.GetLogger().Debug("Using NONMEM category handler")
	case category.CategoryUnknown:
		e.modelCategory = category.NewUnknownCategory()
		category.GetLogger().Warn("Model category unknown - using pass-through handler (execution may fail)")
	case category.CategoryMonolix, category.CategoryStan, category.CategoryTorsten:
		return nil, fmt.Errorf("model category %s is not yet supported for Hermes execution (detected from %s)", categoryType, modelPath)
	default:
		return nil, fmt.Errorf("unexpected model category: %s", categoryType)
	}

	// Generate job ID for audit trail
	var jobID string
	if e.runLogger != nil {
		jobID = e.runLogger.GenerateJobID()
	}

	// Track execution start time
	startTime := time.Now()

	// Run pre-run integration hooks (no-op by default).
	applyPreHooks(ctx, e.config, modelPath)

	// Step 1: Build container structure using category
	modelFiles, err := e.modelCategory.ContainerStructure(modelPath, e.config)
	if err != nil {
		return nil, fmt.Errorf("failed to build container structure: %w", err)
	}

	category.GetLogger().WithFields(map[string]interface{}{
		"file_count": len(modelFiles),
		"category":   e.modelCategory.Name(),
	}).Debug("Container structure built")

	// Step 2: Get license data if category requires it
	var licenseData []byte
	if e.modelCategory.RequiresLicense() {
		licenseData, err = e.modelCategory.GetLicense(e.config)
		if err != nil {
			return nil, fmt.Errorf("failed to get license: %w", err)
		}
		category.GetLogger().Debug("License loaded for category")
	} else {
		category.GetLogger().Debug("Category does not require license, skipping license lookup")
	}

	// Step 3: Build command. In PsN-tool mode run the PsN tool (vpc/scm/…) in the
	// container; otherwise run NONMEM directly.
	var command string
	var args []string
	if e.psnFunction != "" {
		command, args = e.buildPSNCommand(modelPath, additionalOptions)
	} else {
		command, args, err = e.buildNONMEMCommand(modelPath, isParallel, cores, additionalOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to build command: %w", err)
		}
	}

	// Kubernetes orchestrator: provision a pod and reuse the same gRPC transport
	// over a port-forward. Everything below this point is the Docker path.
	if e.usesKubernetes() {
		return e.executeOnKubernetes(ctx, command, args, licenseData, modelFiles, modelPath, jobID, startTime)
	}

	// Step 3.5: Pre-flight cleanup - remove any stale containers for this specific model
	// This is safe because we use labels to identify containers for this exact model path
	e.cleanupStaleContainersForModel(ctx, modelPath)

	// Step 4: Create and start container
	containerID, containerPort, hermesConfigPath, err := e.createContainer(ctx, modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Track whether execution succeeded (for conditional cleanup)
	executionSucceeded := false

	// Clean up Hermes config file after execution completes
	// This defer runs AFTER the container cleanup defer below
	// Ensuring Docker has mounted the file before we delete it
	if hermesConfigPath != "" {
		defer func() {
			if err := os.Remove(hermesConfigPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Warning: Failed to remove Hermes config file %s: %v", hermesConfigPath, err)
			} else {
				category.GetLogger().WithFields(map[string]interface{}{
					"config_path": hermesConfigPath,
				}).Debug("Cleaned up Hermes config file")
			}
		}()
	}

	// Ensure container cleanup (belt and suspenders approach)
	// Even though AutoRemove is set, we explicitly cleanup to handle edge cases
	// IMPORTANT: Only cleanup on SUCCESS - leave container running on failure for debugging
	defer func() {
		category.GetLogger().WithFields(map[string]interface{}{
			"container_id":        containerID[:12],
			"execution_succeeded": executionSucceeded,
		}).Debug("Cleanup defer triggered")

		if e.config == nil {
			category.GetLogger().Debug("e.config is nil, skipping cleanup")

			return
		}

		// Skip cleanup on failure - leave container for debugging
		if !executionSucceeded {
			log.Printf("Execution failed - leaving container %s running for debugging", containerID[:12])
			log.Printf("To view logs: docker logs %s", containerID[:12])
			log.Printf("To inspect: docker exec -it %s /bin/sh", containerID[:12])
			log.Printf("To cleanup manually: docker rm -f %s", containerID[:12])

			return
		}

		category.GetLogger().WithFields(map[string]interface{}{
			"cleanup_enabled": e.config.Hermes.Container.Cleanup,
		}).Debug("Checking cleanup config")

		if e.config.Hermes.Container.Cleanup {
			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
			}).Debug("Attempting cleanup")

			if err := e.cleanupContainer(ctx, containerID); err != nil {
				// Ignore "already removed" errors - this is expected with AutoRemove
				if !strings.Contains(err.Error(), "already") && !strings.Contains(err.Error(), "No such container") {
					log.Printf("Warning: Failed to cleanup container %s: %v", containerID, err)
				} else {
					category.GetLogger().WithFields(map[string]interface{}{
						"container_id": containerID[:12],
					}).Debug("Container already removed (expected with AutoRemove)")
				}
			} else {
				category.GetLogger().WithFields(map[string]interface{}{
					"container_id": containerID[:12],
				}).Debug("Successfully cleaned up container")
			}
		} else {
			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
			}).Debug("Cleanup disabled by config, skipping removal")
		}
	}()

	// Step 6: Wait for container to be healthy
	if err := e.waitForContainer(ctx, containerPort); err != nil {
		return nil, fmt.Errorf("container failed to become healthy: %w", err)
	}

	// Step 7: Execute via gRPC over the Docker container's published port.
	endpoint := hermesEndpoint{
		addr:      fmt.Sprintf("localhost:%d", containerPort),
		id:        containerID,
		displayID: safeShortID(containerID),
		image:     e.buildImageMetadata(ctx, e.modelConfig.Image),
	}

	result, err := e.executeViaGRPC(ctx, command, args, licenseData, modelFiles, endpoint, modelPath, jobID, startTime)
	if err != nil {
		return nil, fmt.Errorf("execution via gRPC failed: %w", err)
	}

	// Mark execution as succeeded so cleanup runs
	executionSucceeded = true

	// Run post-run integration hooks (e.g. R diagnostics). No-op by default.
	applyPostHooks(ctx, e.config, modelPath)

	return result, nil
}

// readLicenseFile reads the NONMEM license file from the configured path.
// If no config is provided, uses sensible defaults in order:
//  1. Config: nonmem.license.path (or deprecated hermes.license.path)
//  2. Home directory: ~/nonmem.lic
//  3. Current directory: ./nonmem.lic
//
// Deprecated: This method has been replaced by category.NONMEMCategory.GetLicense().
// It is kept for backward compatibility but should not be used in new code.
//
//nolint:unused // kept for backward compatibility
func (e *HermesExecutor) readLicenseFile() ([]byte, error) {
	var licensePath string

	// Priority 1: Use new config location (nonmem.license.path)
	switch {
	case e.config != nil && e.config.NONMEM.License.Path != "":
		licensePath = e.config.NONMEM.License.Path
	case e.config != nil && e.config.Hermes.License.Path != "":
		// Backward compatibility: use deprecated hermes.license.path
		licensePath = e.config.Hermes.License.Path
	default:
		// Use sensible defaults when no config provided (standalone executor mode)
		// Priority: 1. ~/nonmem.lic (default), 2. ./nonmem.lic
		home, err := os.UserHomeDir()
		if err == nil {
			homePath := filepath.Join(home, "nonmem.lic")
			if _, statErr := os.Stat(homePath); statErr == nil {
				licensePath = homePath
			}
		}

		if licensePath == "" {
			licensePath = "./nonmem.lic"
		}
	}

	// Expand home directory if needed (handles ~/path syntax)
	if len(licensePath) > 0 && licensePath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to expand home directory: %w", err)
		}
		licensePath = filepath.Join(home, licensePath[1:])
	}

	// Read license file
	licenseData, err := os.ReadFile(licensePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read license file at %s: %w", licensePath, err)
	}

	return licenseData, nil
}

// collectModelFiles collects all files in the model directory for containerized execution.
// This includes the model file, data files, and any supporting files.
//
// Deprecated: This method has been replaced by category.ModelCategory.ContainerStructure().
// It is kept for backward compatibility but should not be used in new code.
//
//nolint:unused // kept for backward compatibility
func (e *HermesExecutor) collectModelFiles(modelPath string) (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Get model directory
	modelDir := filepath.Dir(modelPath)
	modelFileName := filepath.Base(modelPath)

	// Walk the model directory and collect all files
	err := filepath.Walk(modelDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Store with relative path from model directory
		relPath, err := filepath.Rel(modelDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Skip .janus.config.json (Janus metadata, not needed by NONMEM)
		if relPath == ".janus.config.json" {
			return nil
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		files[relPath] = data

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk model directory: %w", err)
	}

	// Ensure model file is included
	if _, ok := files[modelFileName]; !ok {
		return nil, fmt.Errorf("model file %s not found in collected files", modelFileName)
	}

	return files, nil
}

// generateHermesConfig creates a temporary Hermes config file with command overrides.
// Returns the path to the generated config file.
func (e *HermesExecutor) generateHermesConfig() (string, error) {
	// Create temp file in home directory (Docker Desktop requires shared paths)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	tmpDir := filepath.Join(homeDir, ".janus", "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "hermes-config-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer tmpFile.Close()

	// Build Hermes config YAML (shared with the Kubernetes ConfigMap path).
	configYAML := hermesOverrideConfigYAML(e.modelConfig.GetCommandPath())

	// Write config to file
	if _, err := tmpFile.WriteString(configYAML); err != nil {
		os.Remove(tmpFile.Name())

		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	log.Printf("Generated Hermes config at %s (nonmem -> %s)", tmpFile.Name(), e.modelConfig.GetCommandPath())

	return tmpFile.Name(), nil
}

// buildNONMEMCommand constructs the command and arguments for model execution inside the container.
// Applies category-specific command overrides to handle platform differences.
//
//nolint:unparam // error return maintained for interface consistency with other executor implementations
func (e *HermesExecutor) buildNONMEMCommand(modelPath string, isParallel bool, _ int, additionalOptions []string) (string, []string, error) {
	// Always use "nonmem" as the command - Hermes will translate it based on its config
	command := "nonmem"

	// Build arguments - NONMEM expects: nonmem infile outfile -licfile=<path> [options]
	// Use ${WORKSPACE} macro which Hermes expands to the actual workspace directory
	modelFileName := filepath.Base(modelPath)
	outputFile := strings.TrimSuffix(modelFileName, filepath.Ext(modelFileName)) + ".lst"

	// Get category-specific command overrides
	overrides := e.modelCategory.CommandOverrides(e.config)

	// Build base arguments with category overrides
	var args []string
	args = append(args, modelFileName, outputFile)

	// Add license flag if category provides it
	if licenseFlag, ok := overrides["LICENSE_FLAG"]; ok {
		args = append(args, licenseFlag)
		category.GetLogger().WithFields(map[string]interface{}{
			"license_flag": licenseFlag,
			"category":     e.modelCategory.Name(),
		}).Debug("Applied category license flag override")
	}

	// Add parallel arguments if needed (PARAFILE flag for .pnm file)
	if isParallel {
		pnmFile := strings.TrimSuffix(modelFileName, filepath.Ext(modelFileName)) + ".pnm"
		args = append(args, fmt.Sprintf("-PARAFILE=%s", pnmFile))
	}

	// Add any additional options
	if len(additionalOptions) > 0 {
		args = append(args, additionalOptions...)
	}

	category.GetLogger().WithFields(map[string]interface{}{
		"command":  command,
		"args":     args,
		"category": e.modelCategory.Name(),
	}).Debug("Built command with category overrides")

	return command, args, nil
}

// psnHermesDir is the PsN run directory inside the container workspace. It is
// fixed so the executor can retain it deterministically.
const psnHermesDir = "psn_janus"

// psnHermesRetain returns the retain patterns for a PsN-tool run: the whole PsN
// run directory, which holds the analysis artifacts (vpc_results.csv,
// raw_results_*.csv, scmlog.txt, …).
func psnHermesRetain() []string {
	return []string{psnHermesDir + "/**"}
}

// resolveRetainPatterns selects the retain globs for this run by precedence:
//
//  1. PsN-tool mode — the whole PsN run directory (psn_janus/**). Structural, so
//     per-model retain is intentionally NOT consulted here; a user pattern would
//     miss PsN's nested run subdirs. (The bootstrap saga likewise keeps its own
//     hardcoded stage retains and never reaches this method.)
//  2. Per-model .janus.config.json retain — authoritative when set (#94).
//  3. Global hermes.retain — the inherited default when the model has none.
//  4. Category default — the last-resort fallback (config.DefaultNONMEMRetain via
//     the NONMEM category, or "*" for unknown).
func (e *HermesExecutor) resolveRetainPatterns() []string {
	if e.psnFunction != "" {
		return psnHermesRetain()
	}

	var global []string
	if e.config != nil {
		global = e.config.Hermes.Retain
	}

	return config.ResolveRetain(e.retain, global, e.modelCategory.RetentionTargets())
}

// buildPSNCommand builds the command for running a PsN tool (vpc/scm/…) in the
// container: the tool name as the command, then the model file and the form/extra
// args. Any caller-supplied output-directory flag is dropped in favour of a fixed
// directory so the results can be retained deterministically.
func (e *HermesExecutor) buildPSNCommand(modelPath string, additionalOptions []string) (string, []string) {
	modelFileName := filepath.Base(modelPath)

	args := []string{modelFileName}
	for _, a := range append(append([]string{}, e.psnArgs...), additionalOptions...) {
		if strings.HasPrefix(a, "-directory=") || strings.HasPrefix(a, "-dir=") {
			continue
		}

		args = append(args, a)
	}

	args = append(args, "-directory="+psnHermesDir)

	// PsN invokes NONMEM itself (no top-level -licfile), so its nmfe would fall
	// back to the image's install-dir license. Forward the shipped workspace
	// license down to every nmfe call via -nmfe_options. ${WORKSPACE} is expanded
	// by Hermes to the container workspace where ContainerStructure places
	// nonmem.lic; the absolute path lets nmfe find it from PsN's run subdirectories.
	if e.modelCategory != nil && e.modelCategory.RequiresLicense() {
		args = append(args, "-nmfe_options=-licfile=${WORKSPACE}/nonmem.lic")
	}

	return e.psnFunction, args
}

// createContainer creates and starts a Hermes container using Docker SDK.
// Returns the container ID, the port the gRPC service is listening on, and the path to the generated Hermes config file (if any).
func (e *HermesExecutor) createContainer(ctx context.Context, modelPath string) (string, int, string, error) {
	if e.dockerClient == nil {
		return "", 0, "", fmt.Errorf("docker client not initialized - check Docker daemon is running")
	}

	// Get configuration
	image := e.modelConfig.Image

	var configuredPort int
	if e.config != nil {
		configuredPort = e.config.Hermes.Container.Port
	}
	if configuredPort == 0 {
		configuredPort = 50051 // Default gRPC port
	}

	// Generate Hermes config file if command path is specified
	var hermesConfigPath string
	var err error
	if e.modelConfig.GetCommandPath() != "" {
		hermesConfigPath, err = e.generateHermesConfig()
		if err != nil {
			return "", 0, "", fmt.Errorf("failed to generate Hermes config: %w", err)
		}
		// NOTE: Config file cleanup is deferred until AFTER container execution
		// (see Execute cleanup defer). This ensures Docker has time to
		// mount the file before we delete it (critical on macOS Docker Desktop).
	}

	// Generate unique container name
	containerName := fmt.Sprintf("hermes-%d", time.Now().Unix())

	// Get absolute model path and directory for labels
	absModelPath, _ := filepath.Abs(modelPath)
	modelDir := filepath.Dir(absModelPath)
	categoryName := "unknown"
	if e.modelCategory != nil {
		categoryName = e.modelCategory.Name()
	}

	// Create container configuration with labels for identification
	containerConfig := &container.Config{
		Image: image,
		Cmd:   []string{"serve"}, // Hermes serve command
		ExposedPorts: nat.PortSet{
			"50051/tcp": struct{}{},
		},
		Labels: map[string]string{
			"io.github.shairozan.janus.managed":   "true",
			"io.github.shairozan.janus.model":     absModelPath,
			"io.github.shairozan.janus.directory": modelDir,
			"io.github.shairozan.janus.category":  categoryName,
			"io.github.shairozan.janus.executor":  "hermes",
		},
	}

	// Set HERMES_CONFIG environment variable if we generated a config
	if hermesConfigPath != "" {
		containerConfig.Env = []string{
			"HERMES_CONFIG=/etc/hermes/config.yaml",
		}
	}

	// Create host configuration with port binding
	cleanup := true // Default to cleanup
	if e.config != nil {
		cleanup = e.config.Hermes.Container.Cleanup
	}

	hostConfig := &container.HostConfig{
		AutoRemove: cleanup, // Matches --rm flag
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: fmt.Sprintf("%d", configuredPort),
				},
			},
		},
	}

	// Mount Hermes config file if generated
	if hermesConfigPath != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:/etc/hermes/config.yaml:ro", hermesConfigPath),
		}
	}

	// Create container
	resp, err := e.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil, // network config
		nil, // platform
		containerName,
	)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID

	// Start container
	if err := e.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		// Cleanup: try to remove the created but not started container
		_ = e.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

		return "", 0, "", fmt.Errorf("failed to start container: %w", err)
	}

	return containerID, configuredPort, hermesConfigPath, nil
}

// waitForContainer waits for the Hermes container to become healthy and accept gRPC connections.
func (e *HermesExecutor) waitForContainer(ctx context.Context, port int) error {
	timeout := 30 * time.Second
	if e.config != nil && e.config.Hermes.Container.StartupTimeout != "" {
		var err error
		timeout, err = time.ParseDuration(e.config.Hermes.Container.StartupTimeout)
		if err != nil {
			log.Printf("Warning: Invalid startup timeout %s, using 30s default", e.config.Hermes.Container.StartupTimeout)
			timeout = 30 * time.Second
		}
	}

	// Create timeout context
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll for health
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-healthCtx.Done():
			return fmt.Errorf("timeout waiting for container to become healthy")

		case <-ticker.C:
			// Try to connect and health check
			if e.checkHealth(port) {
				return nil
			}
		}
	}
}

// checkHealth performs a health check against the Hermes gRPC service.
func (e *HermesExecutor) checkHealth(port int) bool {
	// Create gRPC connection
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(2*time.Second)) //nolint:staticcheck // SA1019: grpc.Dial is deprecated but supported through 1.x
	if err != nil {
		return false
	}
	defer conn.Close()

	// TODO: Call Health RPC when we have the generated client
	// For now, just successful connection is enough
	return true
}

// monitorContainerHealth monitors the container's running state during execution.
// Sends errors to the provided channel if the container stops unexpectedly.
//
//nolint:unused // reserved for future container monitoring implementation
func (e *HermesExecutor) monitorContainerHealth(ctx context.Context, containerID string, errChan chan<- error) {
	if e.dockerClient == nil {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():

			return

		case <-ticker.C:
			// Inspect container state
			inspect, err := e.dockerClient.ContainerInspect(ctx, containerID)
			if err != nil {
				errChan <- fmt.Errorf("failed to inspect container: %w", err)

				return
			}

			// Check if container is still running
			if !inspect.State.Running {
				// Container stopped - get logs for debugging
				logs, _ := e.getContainerLogs(ctx, containerID)

				if inspect.State.ExitCode != 0 {
					errChan <- fmt.Errorf("container crashed (exit code %d): %s", inspect.State.ExitCode, logs)
				} else {
					errChan <- fmt.Errorf("container stopped unexpectedly (exit code 0)")
				}

				return
			}
		}
	}
}

// getContainerLogs retrieves the last 100 lines of container logs for debugging.
//
//nolint:unused // used internally by monitorContainerHealth
func (e *HermesExecutor) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	if e.dockerClient == nil {
		return "", fmt.Errorf("docker client not initialized")
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100", // Last 100 lines
	}

	reader, err := e.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)

	return string(logs), err
}

// executeViaGRPC executes the command via Hermes gRPC service.
func (e *HermesExecutor) executeViaGRPC(ctx context.Context, command string, args []string, licenseData []byte, modelFiles map[string][]byte, ep hermesEndpoint, modelPath string, jobID string, startTime time.Time) (*ExecutionResult, error) {
	retainPatterns := e.resolveRetainPatterns()
	category.GetLogger().WithFields(map[string]interface{}{
		"category": e.modelCategory.Name(),
		"patterns": retainPatterns,
	}).Debug("Resolved retention patterns")

	// Resolve the optional execution timeout from the global Hermes config.
	var timeout time.Duration
	if e.config != nil && e.config.Hermes.Resources.Timeout != "" {
		if parsed, perr := time.ParseDuration(e.config.Hermes.Resources.Timeout); perr == nil {
			timeout = parsed
		}
	}

	log.Printf("Hermes execution request - Command: %s, Retain patterns: %v", command, retainPatterns)

	// Run via the shared transport; collected files are written to the model dir.
	outcome, err := streamHermesExecution(ctx, ep, hermesStage{
		command:   command,
		args:      args,
		license:   licenseData,
		files:     modelFiles,
		retain:    retainPatterns,
		outputDir: filepath.Dir(modelPath),
		cpuCores:  e.modelConfig.Resources.CPUCores,
		memory:    e.modelConfig.Resources.Memory,
		timeout:   timeout,
	}, e.stdoutWriter, e.stderrWriter)
	if err != nil {
		return nil, err
	}

	// Calculate total duration
	duration := time.Since(startTime)

	// Image metadata + workload identifiers are provided by the orchestrator
	// (Docker image inspect, or the Kubernetes pod's resolved image).
	imageMetadata := ep.image
	shortID := ep.displayID

	// Build container provenance for the run record (CFR 21 Part 11 compliance)
	containerProvenance := &runlog.ContainerProvenance{
		ImageName:      imageMetadata.Name,
		ImageTag:       imageMetadata.Tag,
		ImageDigest:    imageMetadata.Digest,
		ImageFull:      imageMetadata.Full,
		ContainerID:    shortID,
		ExecutionID:    outcome.executionID,
		CPUCores:       e.modelConfig.Resources.CPUCores,
		Memory:         e.modelConfig.Resources.Memory,
		RuntimeSeconds: outcome.runtimeSeconds,
	}

	// Log to audit trail with full Hermes metadata (REQ-50, REQ-51, REQ-52)
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		// Build Hermes metadata for audit trail (includes additional fields)
		hermesMetadata := &runlog.HermesMetadata{
			ExecutionID: outcome.executionID,
			ContainerID: ep.id,
			Image:       *imageMetadata,
			Resources: runlog.HermesResources{
				CPUCores: e.modelConfig.Resources.CPUCores,
				Memory:   e.modelConfig.Resources.Memory,
			},
			ModelConfig: filepath.Join(filepath.Dir(modelPath), ".janus.config.json"),
			FilesCount:  outcome.filesCollected,
			RuntimeSec:  outcome.runtimeSeconds,
		}

		// Record execution with complete metadata
		err := e.runLogger.RecordHermesExecution(
			jobID,
			command,
			args,
			string(outcome.stdout),
			string(outcome.stderr),
			outcome.exitCode,
			duration,
			"/workspace", // Container working directory
			hermesMetadata,
			outcome.outputPaths,
		)
		if err != nil {
			log.Printf("Warning: Failed to record Hermes execution to runlog: %v", err)
		} else {
			log.Printf("Recorded Hermes execution to runlog (job: %s, container: %s, exit: %d, duration: %s)",
				jobID, shortID, outcome.exitCode, duration)
		}
	}

	return &ExecutionResult{
		ExitCode:  outcome.exitCode,
		Stdout:    outcome.stdout,
		Stderr:    outcome.stderr,
		Container: containerProvenance,
	}, nil
}

// parseImageReference parses a Docker image reference into name and tag.
// Handles registry ports correctly by only splitting on the LAST colon.
// Examples:
//   - "shairozan/hermes-nonmem:nm76" -> ("shairozan/hermes-nonmem", "nm76")
//   - "shairozan/hermes-nonmem" -> ("shairozan/hermes-nonmem", "latest")
//   - "nginx:1.21" -> ("nginx", "1.21")
//   - "registry.com:5000/image:tag" -> ("registry.com:5000/image", "tag")
//   - "localhost:5000/test:v1" -> ("localhost:5000/test", "v1")
func (e *HermesExecutor) parseImageReference(imageRef string) (name string, tag string) {
	// Find the last colon to handle registry ports like "registry.com:5000/image:tag"
	lastColonIndex := strings.LastIndex(imageRef, ":")

	// If no colon found, or colon is before a slash (part of registry port), default to latest
	if lastColonIndex == -1 {
		return imageRef, "latest"
	}

	// Check if there's a slash after the last colon - if so, it's part of the path, not a tag separator
	slashAfterColon := strings.Index(imageRef[lastColonIndex:], "/")
	if slashAfterColon != -1 {
		// Colon is part of registry port (e.g., "registry:5000/image"), not a tag separator
		return imageRef, "latest"
	}

	// Split at the last colon
	name = imageRef[:lastColonIndex]
	tag = imageRef[lastColonIndex+1:]

	// If tag is empty, default to latest
	if tag == "" {
		return name, "latest"
	}

	return name, tag
}

// getImageDigest retrieves the SHA256 digest for a Docker image using Docker SDK.
// Returns empty string if digest cannot be retrieved.
func (e *HermesExecutor) getImageDigest(ctx context.Context, imageRef string) string {
	if e.dockerClient == nil {
		log.Printf("Warning: Docker client not initialized, cannot get image digest")

		return ""
	}

	// Inspect image to get digest
	inspect, _, err := e.dockerClient.ImageInspectWithRaw(ctx, imageRef) //nolint:staticcheck // SA1019: will migrate to ImageInspect in future
	if err != nil {
		log.Printf("Warning: Failed to inspect image %s: %v", imageRef, err)

		return ""
	}

	// Get RepoDigests - these are the actual registry digests
	if len(inspect.RepoDigests) > 0 {
		// Format: "repository/image@sha256:abc123..."
		// Extract just the SHA256 part
		fullDigest := inspect.RepoDigests[0]
		if strings.Contains(fullDigest, "@") {
			parts := strings.Split(fullDigest, "@")
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}

	// Fallback to image ID (local digest)
	// Remove "sha256:" prefix if present
	imageID := strings.TrimPrefix(inspect.ID, "sha256:")
	if imageID != "" {
		return "sha256:" + imageID
	}

	return ""
}

// buildImageMetadata creates complete container image provenance metadata.
// Implements REQ-51: Container image provenance logging.
func (e *HermesExecutor) buildImageMetadata(ctx context.Context, imageRef string) *runlog.ContainerImage {
	name, tag := e.parseImageReference(imageRef)
	digest := e.getImageDigest(ctx, imageRef)

	return &runlog.ContainerImage{
		Name:   name,
		Tag:    tag,
		Digest: digest,
		Full:   imageRef,
	}
}

// cleanupContainer stops and removes the Hermes container using Docker SDK.
//
//nolint:contextcheck // Intentionally creates new context - original may be cancelled but cleanup must complete
func (e *HermesExecutor) cleanupContainer(_ context.Context, containerID string) error {
	category.GetLogger().WithFields(map[string]interface{}{
		"container_id": containerID[:12],
	}).Debug("cleanupContainer called")

	if e.dockerClient == nil {
		category.GetLogger().Error("Docker client is nil, cannot cleanup")

		return fmt.Errorf("docker client not initialized")
	}

	// Create cleanup context with timeout to prevent hanging
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	category.GetLogger().WithFields(map[string]interface{}{
		"container_id": containerID[:12],
	}).Debug("Stopping container")

	// Stop container (with grace period)
	stopTimeout := 10 // seconds
	if err := e.dockerClient.ContainerStop(cleanupCtx, containerID, container.StopOptions{
		Timeout: &stopTimeout,
	}); err != nil {
		// Swallow "container not running" and "no such container" errors
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "is not running") &&
			!strings.Contains(errMsg, "no such container") &&
			!strings.Contains(errMsg, "already stopped") {

			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
				"error":        err.Error(),
			}).Warn("Failed to stop container")
		} else {
			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
				"error":        err.Error(),
			}).Debug("Container not running (expected)")
		}
		// Continue to try removal even if stop fails
	} else {
		category.GetLogger().WithFields(map[string]interface{}{
			"container_id": containerID[:12],
		}).Debug("Container stopped successfully")
	}

	category.GetLogger().WithFields(map[string]interface{}{
		"container_id": containerID[:12],
	}).Debug("Removing container")

	// Remove container (force if needed)
	if err := e.dockerClient.ContainerRemove(cleanupCtx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		// Swallow errors that indicate the container is already gone or being removed
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "no such container") ||
			strings.Contains(errMsg, "already") ||
			strings.Contains(errMsg, "removal of container") && strings.Contains(errMsg, "is already in progress") {

			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
				"error":        err.Error(),
			}).Debug("Container already removed or being removed")

			return nil
		}

		category.GetLogger().WithFields(map[string]interface{}{
			"container_id": containerID[:12],
			"error":        err.Error(),
		}).Error("Failed to remove container")

		return fmt.Errorf("failed to remove container %s: %w", containerID[:12], err)
	}

	category.GetLogger().WithFields(map[string]interface{}{
		"container_id": containerID[:12],
	}).Debug("Container removed successfully")

	return nil
}

// cleanupStaleContainersForModel removes any Janus-managed containers for the specified model.
// This is safe because we only remove containers with specific labels matching this model path.
// Handles cases where previous executions crashed or were killed before cleanup could run.
func (e *HermesExecutor) cleanupStaleContainersForModel(ctx context.Context, modelPath string) {
	if e.dockerClient == nil {
		return
	}

	// Get absolute model path for matching
	absModelPath, err := filepath.Abs(modelPath)
	if err != nil {
		category.GetLogger().WithFields(map[string]interface{}{
			"model_path": modelPath,
			"error":      err.Error(),
		}).Warn("Failed to get absolute model path for cleanup")

		return
	}

	// List containers with our management label and matching model path
	filters := filters.NewArgs()
	filters.Add("label", "io.github.shairozan.janus.managed=true")
	filters.Add("label", fmt.Sprintf("io.github.shairozan.janus.model=%s", absModelPath))

	containers, err := e.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: filters,
	})
	if err != nil {
		category.GetLogger().WithFields(map[string]interface{}{
			"model_path": absModelPath,
			"error":      err.Error(),
		}).Warn("Failed to list containers for cleanup")

		return
	}

	if len(containers) == 0 {
		category.GetLogger().WithFields(map[string]interface{}{
			"model_path": absModelPath,
		}).Debug("No stale containers found for model")

		return
	}

	// Clean up each stale container
	for _, c := range containers {
		containerID := c.ID
		category.GetLogger().WithFields(map[string]interface{}{
			"container_id": containerID[:12],
			"model_path":   absModelPath,
			"status":       c.State,
		}).Info("Cleaning up stale container for model")

		// Use the existing cleanup method
		if err := e.cleanupContainer(ctx, containerID); err != nil {
			category.GetLogger().WithFields(map[string]interface{}{
				"container_id": containerID[:12],
				"error":        err.Error(),
			}).Warn("Failed to cleanup stale container")
		}
	}
}
