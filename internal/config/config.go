package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/shairozan/janus/internal/scheduler"
	"github.com/shairozan/janus/internal/version"
)

// Execution mode constants.
//
// ExecutionMode is the legacy, flat selector the executor factory still
// dispatches on. It is derived from the orthogonal Engine/Destination axes
// (see normalizeExecutionAxes) and kept in sync for backward compatibility.
const (
	ExecutionModeNONMEM = "NONMEM"
	ExecutionModeBBI    = "BBI"
	ExecutionModePSN    = "PSN"
	ExecutionModeHERMES = "HERMES"
)

// Engine constants — the "what": the command structure used to invoke a NONMEM
// run. These are peers; BBI is simply another command structure, not a
// destination. Engine is orthogonal to Destination.
const (
	EngineNONMEM = "NONMEM"
	EnginePSN    = "PSN"
	EngineBBI    = "BBI"
)

// Destination constants — the "where": where the engine's command actually
// runs. Hermes fans out further into an Orchestrator (Docker/Kubernetes).
const (
	DestinationHere      = "Here"
	DestinationScheduler = "Scheduler"
	DestinationHermes    = "Hermes"
)

// Orchestrator constants — the Hermes sub-choice. Kubernetes is not yet
// executable (tracked in issue #13); only Docker runs today.
const (
	OrchestratorDocker     = "Docker"
	OrchestratorKubernetes = "Kubernetes"
)

// Input represents configuration that comes from viper (files, flags, env vars).
type Input struct {
	// User Configuration
	Organization     string `mapstructure:"organization" yaml:"organization"`
	DefaultDirectory string `mapstructure:"default-directory" yaml:"default-directory"`
	NonmemPath       string `mapstructure:"nonmem-path" yaml:"nonmem-path"`
	NonmemBinary     string `mapstructure:"nonmem-binary" yaml:"nonmem-binary"`

	// Installations lists named NONMEM installations. When set, the default
	// entry populates nonmem-path/nonmem-binary (above) so all executors use it
	// transparently. The single nonmem-path/binary remains the implicit default
	// when no installations are configured (backward compatible).
	Installations []NonmemInstall `mapstructure:"installations" yaml:"installations"`

	// General run preferences (Pirana parity).
	// RunPrefix is prepended to generated run/job names.
	RunPrefix string `mapstructure:"run-prefix" yaml:"run-prefix"`
	// AltDataDirectory is an additional search directory for model data files
	// when they are not found next to the model.
	AltDataDirectory string `mapstructure:"alt-data-directory" yaml:"alt-data-directory"`
	// CloseConsoleAfterRun asks the GUI to close the run console after a run
	// finishes.
	CloseConsoleAfterRun bool `mapstructure:"close-console-after-run" yaml:"close-console-after-run"`

	// Scheduler Configuration
	Scheduler string `mapstructure:"scheduler" yaml:"scheduler"`

	// Schedulers holds optional workload-manager profile definitions. Built-in
	// SLURM/SGE/Torque profiles are used when this is empty; entries here add or
	// override a profile by Name. Validated lazily — resolved at execution time,
	// not at startup — so the headless executor never rejects a config that
	// merely declares extra schedulers.
	Schedulers []scheduler.Profile `mapstructure:"schedulers" yaml:"schedulers"`

	// Execution Configuration
	//
	// Engine/Destination/Orchestrator are the orthogonal axes the GUI presents.
	// ExecutionMode is the derived, legacy selector the executor factory
	// dispatches on; normalizeExecutionAxes keeps them consistent in both
	// directions so older configs (ExecutionMode-only) and the new axes both
	// load correctly.
	Engine        string `mapstructure:"engine" yaml:"engine"`
	Destination   string `mapstructure:"destination" yaml:"destination"`
	Orchestrator  string `mapstructure:"orchestrator" yaml:"orchestrator"`
	ExecutionMode string `mapstructure:"execution-mode" yaml:"execution-mode"`

	// Feature Flags
	RunLogEnabled  bool `mapstructure:"runlog-enabled" yaml:"runlog-enabled"`
	ProjectsEnable bool `mapstructure:"projects" yaml:"projects"`

	// Run-output directory policy (overwrite protection, backup, cleanup).
	// Defaults are all no-ops, so behavior is unchanged unless opted in.
	Runs RunsConfig `mapstructure:"runs" yaml:"runs"`

	// Remote (SSH) execution. Empty Host means local execution. Validated
	// lazily so a headless executor never rejects a config that declares it.
	Remote RemoteConfig `mapstructure:"remote" yaml:"remote"`

	// PsN integration settings (path, psn.conf, presets, pre/post hooks).
	PSN PSNConfig `mapstructure:"psn" yaml:"psn"`

	// Software-integration settings: R path, generic pre/post-run script hooks
	// (e.g. R diagnostics), and named external tool paths (Stan, NMQual, WFN).
	Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`

	// Validation Configuration for CFR 21 Part 11 compliance
	Validation ValidationControl `mapstructure:"validation" yaml:"validation"`

	// Future settings (commented in YAML but struct ready)
	SLURM        SLURMConfig    `mapstructure:"slurm" yaml:"slurm"`
	RunLog       RunLogConfig   `mapstructure:"runlog" yaml:"runlog"`
	ProjectsConf ProjectsConfig `mapstructure:"projects_config" yaml:"projects_config"`
	Hermes       HermesConfig   `mapstructure:"hermes" yaml:"hermes"`

	// NONMEM-specific configuration
	NONMEM NONMEMConfig `mapstructure:"nonmem" yaml:"nonmem"`

	// Run log signing configuration
	Signing SigningConfig `mapstructure:"signing" yaml:"signing"`

	// Embedded MCP server configuration
	MCP MCPConfig `mapstructure:"mcp" yaml:"mcp"`
}

// MCPConfig configures the embedded Model Context Protocol server that lets
// local agents (e.g. Claude Code) query the run log and launch runs over
// loopback. It is disabled by default; see documentation/features/mcp.md.
type MCPConfig struct {
	// Enabled turns the MCP server on. Default false.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// Host is the bind address. Loopback only (127.0.0.1 / localhost / ::1);
	// non-loopback hosts are rejected by ValidateMCPConfig. Default 127.0.0.1.
	Host string `mapstructure:"host" yaml:"host"`

	// Port is the TCP port to bind. Default 8731; must be in 1024-65535.
	Port int `mapstructure:"port" yaml:"port"`

	// AuthToken is the mandatory bearer token. When empty, Janus generates one
	// (crypto/rand) on first start and persists it back to the config file, so it
	// stays stable across restarts. See documentation/features/mcp.md.
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token"`

	// AllowExecute gates the execute_run tool. Default false; the tool is not even
	// registered unless this is set, on top of Enabled and the bearer token.
	AllowExecute bool `mapstructure:"allow_execute" yaml:"allow_execute"`
}

// Config represents the complete runtime configuration for the application.
// This includes both viper-sourced configuration and external sources.
type Config struct {
	// External sources (not from viper)
	Version string // From build-time ldflags
	User    string // From OS user

	// Viper-sourced configuration (embedded)
	Input
}

// ValidationControl represents validation configuration for CFR 21 Part 11 compliance.
type ValidationControl struct {
	IQ string `mapstructure:"iq" yaml:"iq"` // Installation Qualification output path
	OQ string `mapstructure:"oq" yaml:"oq"` // Operational Qualification output path
}

// SLURM mode constants.
const (
	SLURMModeREST = "REST"
	SLURMModeCLI  = "CLI"
)

// SLURMConfig represents SLURM-specific configuration.
type SLURMConfig struct {
	// Mode determines how to interact with SLURM: "REST" or "CLI"
	Mode string `mapstructure:"mode" yaml:"mode"`

	// CLI mode configuration (legacy)
	Host    string `mapstructure:"host" yaml:"host"`
	Port    int    `mapstructure:"port" yaml:"port"`
	Timeout string `mapstructure:"timeout" yaml:"timeout"`

	// REST mode configuration
	REST SLURMRESTConfig `mapstructure:"rest" yaml:"rest"`
}

// SLURMRESTConfig represents SLURM REST API configuration.
type SLURMRESTConfig struct {
	// SocketPath is the path to the Unix domain socket for slurmrestd
	SocketPath string `mapstructure:"socket_path" yaml:"socket_path"`

	// APIVersion specifies which OpenAPI specification version to use
	// Common values: "v0.0.40", "v0.0.39", "v0.0.38"
	APIVersion string `mapstructure:"api_version" yaml:"api_version"`

	// Timeout for REST API requests
	Timeout string `mapstructure:"timeout" yaml:"timeout"`

	// AuthToken for SLURM REST API authentication (optional)
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token"`
}

// RunLogConfig represents run log configuration.
type RunLogConfig struct {
	Backend string `mapstructure:"backend" yaml:"backend"`
	Path    string `mapstructure:"path" yaml:"path"`
}

// ProjectsConfig represents project management configuration.
type ProjectsConfig struct {
	DefaultTemplate string `mapstructure:"default-template" yaml:"default-template"`
	AutoBackup      bool   `mapstructure:"auto-backup" yaml:"auto-backup"`
}

// RemoteMount maps a local mount point onto its absolute path on the remote
// host, so local model paths can be translated to remote paths before commands
// are sent over SSH (and results read back locally via the shared mount).
type RemoteMount struct {
	Local  string `mapstructure:"local" yaml:"local"`
	Remote string `mapstructure:"remote" yaml:"remote"`
}

// RemoteConfig configures SSH-based execution on a remote cluster. When Host is
// empty, execution is local. Authentication is key-based only.
type RemoteConfig struct {
	Host    string        `mapstructure:"host" yaml:"host"`
	Port    int           `mapstructure:"port" yaml:"port"`
	User    string        `mapstructure:"user" yaml:"user"`
	KeyPath string        `mapstructure:"key_path" yaml:"key_path"`
	Mounts  []RemoteMount `mapstructure:"mounts" yaml:"mounts"`
}

// RunsConfig controls how a run's output directory is managed: overwrite
// protection (sequential numbered dirs), backup, and intermediate-file cleanup.
type RunsConfig struct {
	// OverwritePolicy is "overwrite" (default) or "sequential".
	OverwritePolicy string `mapstructure:"overwrite_policy" yaml:"overwrite_policy"`

	// AutoBackup copies the model and its results to a backup/ subfolder after
	// a run. The legacy projects_config.auto-backup flag is honored as a
	// fallback (see RunPolicy consumers).
	AutoBackup bool `mapstructure:"auto_backup" yaml:"auto_backup"`

	// CleanupGlobs are globs (relative to the model dir) deleted after a run.
	CleanupGlobs []string `mapstructure:"cleanup_globs" yaml:"cleanup_globs"`
}

// NONMEMLicenseConfig represents NONMEM license file configuration.
type NONMEMLicenseConfig struct {
	// Path is the absolute or home-relative path to the NONMEM license file.
	// If not specified, defaults to ~/nonmem.lic with fallback to ./nonmem.lic.
	// Use this to specify a non-default license location.
	Path string `mapstructure:"path" yaml:"path"`
}

// SigningConfig represents run log cryptographic signing configuration.
//
// Signing and verification are independent halves. PrivateKeyPath and Identity
// describe the key this install signs with; KeyringPath lists the public keys
// whose signatures it accepts. Either half may be configured without the other:
// you can sign without trusting anyone, and verify without holding a key.
//
// All three are optional — signing is off by default.
type SigningConfig struct {
	// Backend selects where the private key lives: SigningBackendKeychain (the OS
	// credential store, keyed by Identity) or SigningBackendFile (PrivateKeyPath).
	//
	// Empty means infer: a configured PrivateKeyPath wins, else a configured
	// Identity implies the keychain, else signing is off. That keeps configs
	// written before the credential store existed working untouched.
	Backend string `mapstructure:"backend" yaml:"backend"`

	// PrivateKeyPath is the path to the RSA private key file (PEM format).
	// This key is used to sign run log entries for cryptographic verification.
	// It is the backend for hosts with no usable credential store — headless
	// servers, containers and CI, where there is no session keyring to unlock.
	PrivateKeyPath string `mapstructure:"private_key_path" yaml:"private_key_path"`

	// Identity labels the signing key — conventionally an email address — and is
	// recorded on every signed record as signer_email. It belongs to the key, not
	// to the process: verification anchors on the key fingerprint, and the keyring
	// maps this label to a public key. It is deliberately not derived from the OS
	// user at run time, which would let the recorded identity drift from the key
	// that produced the signature.
	Identity string `mapstructure:"identity" yaml:"identity"`

	// KeyringPath is the path to the YAML keyring listing the public keys whose
	// signatures this install accepts. A missing keyring means "trust nobody":
	// signed records report Unverifiable rather than Valid.
	KeyringPath string `mapstructure:"keyring_path" yaml:"keyring_path"`
}

// NONMEMConfig represents NONMEM-specific configuration.
type NONMEMConfig struct {
	// License configuration for NONMEM license file location
	License NONMEMLicenseConfig `mapstructure:"license" yaml:"license"`
}

// HermesConfig represents Hermes container execution configuration.
type HermesConfig struct {
	// Image is the Docker/container image to use for Hermes execution
	Image string `mapstructure:"image" yaml:"image"`

	// PsNImage is the default container image for staged PsN orchestration stages
	// (the horizontal-bootstrap resample + aggregate pods). It needs PsN but no
	// NONMEM license. A per-model .janus.config.json psn_image overrides it.
	PsNImage string `mapstructure:"psn_image" yaml:"psn_image"`

	// License configuration for NONMEM license file handling
	License HermesLicenseConfig `mapstructure:"license" yaml:"license"`

	// Resources defines default resource limits for Hermes containers
	Resources HermesResourceConfig `mapstructure:"resources" yaml:"resources"`

	// Container defines container lifecycle and connection settings
	Container HermesContainerConfig `mapstructure:"container" yaml:"container"`

	// Kubernetes configures where Janus provisions the Hermes pod when the
	// execution Orchestrator is "Kubernetes". Image and resources still come from
	// the per-model .janus.config.json.
	Kubernetes HermesKubernetesConfig `mapstructure:"kubernetes" yaml:"kubernetes"`

	// Retain defines glob patterns for files to collect after execution
	Retain []string `mapstructure:"retain" yaml:"retain"`
}

// HermesKubernetesConfig tells Janus where to orchestrate the Hermes pod when
// the Kubernetes orchestrator is selected. The image and resource requirements
// still come from the per-model .janus.config.json — these settings only locate
// the cluster, context, and namespace.
type HermesKubernetesConfig struct {
	// Kubeconfig is the path to the kubeconfig file. Empty uses the standard
	// resolution: the KUBECONFIG environment variable, then ~/.kube/config.
	Kubeconfig string `mapstructure:"kubeconfig" yaml:"kubeconfig"`

	// Context is the kubeconfig context to use. Empty uses the current-context
	// from the kubeconfig.
	Context string `mapstructure:"context" yaml:"context"`

	// Namespace is the namespace the Hermes pod is created in. Required when the
	// Kubernetes orchestrator is selected.
	Namespace string `mapstructure:"namespace" yaml:"namespace"`

	// ImagePullSecret names a docker-registry secret (in Namespace) used to pull
	// the Hermes image from a private registry (e.g. a private GHCR repo). Empty
	// relies on the pod's service account / node credentials. Create it with:
	//   kubectl create secret docker-registry <name> --docker-server=ghcr.io \
	//     --docker-username=<user> --docker-password=<token> -n <namespace>
	ImagePullSecret string `mapstructure:"image_pull_secret" yaml:"image_pull_secret"`

	// BootstrapParallelism caps how many fit pods run concurrently during a
	// horizontal bootstrap — the FIFO sliding-window watermark, and the cost/speed
	// dial (peak node-hours scale with it). Defaults to DefaultBootstrapParallelism
	// when unset (<= 0). See BootstrapParallelismOrDefault.
	BootstrapParallelism int `mapstructure:"bootstrap_parallelism" yaml:"bootstrap_parallelism"`

	// FitTimeout bounds a single bootstrap fit pod's run (e.g. "30m"); a hung fit
	// is abandoned so its slot — and its node — frees. Empty means no per-fit
	// deadline. See FitTimeoutDuration.
	FitTimeout string `mapstructure:"fit_timeout" yaml:"fit_timeout"`
}

// DefaultBootstrapParallelism is the FIFO worker-pool watermark used when
// hermes.kubernetes.bootstrap_parallelism is unset.
const DefaultBootstrapParallelism = 16

// BootstrapParallelismOrDefault returns the configured fit-pod concurrency, or
// DefaultBootstrapParallelism when it is unset (<= 0).
func (k HermesKubernetesConfig) BootstrapParallelismOrDefault() int {
	if k.BootstrapParallelism > 0 {
		return k.BootstrapParallelism
	}

	return DefaultBootstrapParallelism
}

// FitTimeoutDuration parses FitTimeout. It returns 0 (meaning no per-fit
// deadline) when FitTimeout is empty, or an error when it is not a valid Go
// duration string.
func (k HermesKubernetesConfig) FitTimeoutDuration() (time.Duration, error) {
	if strings.TrimSpace(k.FitTimeout) == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(k.FitTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid hermes.kubernetes.fit_timeout %q: %w", k.FitTimeout, err)
	}

	return d, nil
}

// HermesLicenseConfig represents NONMEM license file configuration.
//
// Deprecated: Use NONMEMLicenseConfig (nonmem.license.path) instead.
// This struct is kept for backward compatibility. If hermes.license.path is set
// and nonmem.license.path is empty, the value will be migrated automatically.
type HermesLicenseConfig struct {
	// Path is the absolute or home-relative path to the NONMEM license file
	// This file will be read and sent as bytes in the Hermes execution request
	//
	// Deprecated: Use nonmem.license.path instead
	Path string `mapstructure:"path" yaml:"path"`
}

// HermesResourceConfig represents default resource limits for Hermes execution.
type HermesResourceConfig struct {
	// CPUs is the number of CPUs to allocate (e.g., "4")
	CPUs string `mapstructure:"cpus" yaml:"cpus"`

	// Memory is the memory limit (e.g., "8G")
	Memory string `mapstructure:"memory" yaml:"memory"`

	// Timeout is the maximum execution time (e.g., "24h")
	Timeout string `mapstructure:"timeout" yaml:"timeout"`
}

// HermesContainerConfig represents container lifecycle settings.
type HermesContainerConfig struct {
	// Port is the gRPC port for the Hermes service (default: 50051)
	Port int `mapstructure:"port" yaml:"port"`

	// Cleanup indicates whether to delete containers after execution (default: true)
	Cleanup bool `mapstructure:"cleanup" yaml:"cleanup"`

	// StartupTimeout is the maximum time to wait for container startup (e.g., "30s")
	StartupTimeout string `mapstructure:"startup_timeout" yaml:"startup_timeout"`

	// DockerSocket is the path to the Docker socket (optional)
	// If not specified, uses DOCKER_HOST env var or default /var/run/docker.sock
	// Common values:
	//   - unix:///var/run/docker.sock (default Docker daemon)
	//   - unix:///home/user/.docker/desktop/docker.sock (Docker Desktop on Linux)
	//   - tcp://localhost:2375 (TCP connection)
	DockerSocket string `mapstructure:"docker_socket" yaml:"docker_socket"`
}

// IntegrationsConfig configures software integrations: an R path used to run
// hook scripts, generic pre/post-run script hooks, and named external tools.
type IntegrationsConfig struct {
	// RPath is the Rscript executable or the R bin directory used to run hooks.
	// Empty uses "Rscript" from PATH.
	RPath string `mapstructure:"r_path" yaml:"r_path"`

	// Hooks are scripts run before/after a run (e.g. R diagnostics post-fit).
	Hooks []IntegrationHook `mapstructure:"hooks" yaml:"hooks"`

	// Tools registers named external tool paths (Stan, NMQual, WFN, …).
	Tools []IntegrationTool `mapstructure:"tools" yaml:"tools"`
}

// IntegrationHook is a script run around a run.
type IntegrationHook struct {
	Name string `mapstructure:"name" yaml:"name"`
	// When is "pre" or "post".
	When   string `mapstructure:"when" yaml:"when"`
	Script string `mapstructure:"script" yaml:"script"`
}

// IntegrationTool registers a named external tool path.
type IntegrationTool struct {
	Name string `mapstructure:"name" yaml:"name"`
	Path string `mapstructure:"path" yaml:"path"`
}

// PSNConfig configures PsN integration.
type PSNConfig struct {
	// Path is the PsN bin directory; commands resolve to <Path>/<tool>. Empty
	// uses tools from PATH.
	Path string `mapstructure:"path" yaml:"path"`

	// ConfPath points at a psn.conf to honor (PsN otherwise discovers its own).
	ConfPath string `mapstructure:"conf_path" yaml:"conf_path"`

	// PreScript / PostScript are R scripts run before/after a PsN run.
	PreScript  string `mapstructure:"pre_script" yaml:"pre_script"`
	PostScript string `mapstructure:"post_script" yaml:"post_script"`

	// Presets are named PsN command templates (vpc, bootstrap, scm, …).
	Presets []PSNPreset `mapstructure:"presets" yaml:"presets"`
}

// PSNPreset is a named PsN command template.
type PSNPreset struct {
	Name string   `mapstructure:"name" yaml:"name"`
	Tool string   `mapstructure:"tool" yaml:"tool"`
	Args []string `mapstructure:"args" yaml:"args"`
}

// Preset returns the preset with the given name (case-insensitive), or false.
func (c PSNConfig) Preset(name string) (PSNPreset, bool) {
	for _, p := range c.Presets {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}

	return PSNPreset{}, false
}

// NonmemInstall is a named NONMEM installation: a path and binary the user can
// select per run.
type NonmemInstall struct {
	Name    string `mapstructure:"name" yaml:"name"`
	Path    string `mapstructure:"path" yaml:"path"`
	Binary  string `mapstructure:"binary" yaml:"binary"`
	Default bool   `mapstructure:"default" yaml:"default"`
}

// DefaultNonmemInstallation returns the effective installation: the entry
// flagged default (else the first) from Installations, or a synthetic install
// from the legacy nonmem-path/binary when no list is configured.
func (i Input) DefaultNonmemInstallation() NonmemInstall {
	for _, in := range i.Installations {
		if in.Default {
			return in
		}
	}

	if len(i.Installations) > 0 {
		return i.Installations[0]
	}

	return NonmemInstall{Name: "default", Path: i.NonmemPath, Binary: i.NonmemBinary}
}

// NonmemInstallation returns the installation with the given name (matched
// case-insensitively), or false. The synthetic "default" install is returned
// when no installations are configured.
func (i Input) NonmemInstallation(name string) (NonmemInstall, bool) {
	for _, in := range i.Installations {
		if strings.EqualFold(in.Name, name) {
			return in, true
		}
	}

	if len(i.Installations) == 0 && strings.EqualFold(name, "default") {
		return i.DefaultNonmemInstallation(), true
	}

	return NonmemInstall{}, false
}

// InstallationNames returns the configured installation names, or {"default"}
// when none are configured.
func (i Input) InstallationNames() []string {
	if len(i.Installations) == 0 {
		return []string{"default"}
	}

	names := make([]string, 0, len(i.Installations))
	for _, in := range i.Installations {
		names = append(names, in.Name)
	}

	return names
}

// UnmarshalInputFromViper creates Input from Viper configuration.
func UnmarshalInputFromViper() (*Input, error) {
	input := &Input{}

	// Unmarshal from viper (defaults are handled by cobra flags)
	if err := viper.Unmarshal(input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input config: %w", err)
	}

	return input, nil
}

// NewConfig creates a complete Config by merging Input with external sources.
func NewConfig(input *Input) (*Config, error) {
	// Reconcile the Engine/Destination axes with the legacy ExecutionMode field
	// before anything else reads either, so the rest of NewConfig (and the
	// executor factory) sees a consistent, derived ExecutionMode.
	normalizeExecutionAxes(input)

	// Validate execution mode and required tools
	if err := ValidateExecutionMode(input.ExecutionMode); err != nil {
		return nil, fmt.Errorf("execution mode validation failed: %w", err)
	}

	// Backward compatibility: migrate hermes.license.path to nonmem.license.path
	if input.NONMEM.License.Path == "" && input.Hermes.License.Path != "" {
		log.Printf("DEPRECATION WARNING: 'hermes.license.path' is deprecated. Please use 'nonmem.license.path' instead.")
		input.NONMEM.License.Path = input.Hermes.License.Path
	}

	// When only an installations list is configured, populate the legacy
	// nonmem-path/binary from the default install so every executor (which reads
	// those fields) transparently uses it.
	if input.NonmemPath == "" && len(input.Installations) > 0 {
		def := input.DefaultNonmemInstallation()
		input.NonmemPath = def.Path
		input.NonmemBinary = def.Binary
	}

	// Validate NONMEM configuration (if path specified, verify it exists)
	if err := ValidateNONMEMConfig(input.NONMEM); err != nil {
		return nil, fmt.Errorf("NONMEM configuration validation failed: %w", err)
	}

	// Validate mode-specific configuration
	if input.ExecutionMode == ExecutionModeHERMES {
		if err := ValidateHermesConfig(input.Hermes); err != nil {
			return nil, fmt.Errorf("hermes configuration validation failed: %w", err)
		}

		if err := ValidateHermesKubernetes(input.Orchestrator, input.Hermes.Kubernetes); err != nil {
			return nil, fmt.Errorf("hermes kubernetes configuration validation failed: %w", err)
		}
	}

	// Validate (and default) the embedded MCP server configuration. This is a
	// no-op when disabled, but it normalizes host/port and enforces loopback.
	if err := ValidateMCPConfig(&input.MCP); err != nil {
		return nil, fmt.Errorf("MCP configuration validation failed: %w", err)
	}

	cfg := &Config{
		Version: version.Get(),
		Input:   *input,
	}

	// Populate runtime fields
	if err := populateRuntimeFields(cfg); err != nil {
		return nil, fmt.Errorf("failed to populate runtime fields: %w", err)
	}

	return cfg, nil
}

// populateRuntimeFields fills in fields that are determined at runtime.
func populateRuntimeFields(cfg *Config) error {
	// Get current OS user
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Use username (not display name) for consistency
	cfg.User = currentUser.Username

	return nil
}

// Process creates a complete Config by processing viper configuration and external sources.
// This function combines UnmarshalInputFromViper and NewConfig to provide a complete
// runtime configuration that includes both viper-sourced and external data.
func Process() (*Config, error) {
	input, err := UnmarshalInputFromViper()
	if err != nil {
		return nil, err
	}

	return NewConfig(input)
}

// ValidateExecutionMode validates that the execution mode is a valid option.
// It does NOT check for tool existence - that validation happens at execution time.
func ValidateExecutionMode(mode string) error {
	switch mode {
	case ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN, ExecutionModeHERMES:
		// All valid execution modes - tool existence is validated at execution time
		return nil
	default:
		return fmt.Errorf("invalid execution mode: %q (must be one of: %s, %s, %s, %s)",
			mode, ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN, ExecutionModeHERMES)
	}
}

// GetValidExecutionModes returns a list of valid execution modes.
func GetValidExecutionModes() []string {
	return []string{ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN, ExecutionModeHERMES}
}

// GetValidEngines returns the selectable execution engines (the "what").
func GetValidEngines() []string {
	return []string{EngineNONMEM, EnginePSN, EngineBBI}
}

// GetValidDestinations returns the selectable execution destinations (the
// "where").
func GetValidDestinations() []string {
	return []string{DestinationHere, DestinationScheduler, DestinationHermes}
}

// GetValidOrchestrators returns the Hermes orchestrator options. Kubernetes is
// listed but not yet executable (issue #13).
func GetValidOrchestrators() []string {
	return []string{OrchestratorDocker, OrchestratorKubernetes}
}

// DeriveExecutionMode maps an (engine, destination) pair onto the legacy
// ExecutionMode the executor factory dispatches on. A Hermes destination always
// runs via the HERMES executor regardless of engine — the engine runs inside
// the container — while every other destination runs the engine's own command
// structure on the host.
func DeriveExecutionMode(engine, destination string) string {
	if destination == DestinationHermes {
		return ExecutionModeHERMES
	}

	return engine
}

// axesFromLegacy derives the Engine/Destination/Orchestrator axes from a legacy
// ExecutionMode + Scheduler pair, for configs written before the axes existed.
func axesFromLegacy(mode, sched string) (engine, destination, orchestrator string) {
	if mode == ExecutionModeHERMES {
		return EngineNONMEM, DestinationHermes, OrchestratorDocker
	}

	switch mode {
	case ExecutionModeNONMEM, ExecutionModePSN, ExecutionModeBBI:
		engine = mode
	default:
		engine = EngineNONMEM
	}

	if sched == "" || strings.EqualFold(sched, "LOCAL") {
		destination = DestinationHere
	} else {
		destination = DestinationScheduler
	}

	return engine, destination, ""
}

// normalizeExecutionAxes reconciles the Engine/Destination axes with the legacy
// ExecutionMode/Scheduler fields, in whichever direction is needed:
//
//   - Legacy-only config (no axes set): derive the axes for display, but leave
//     ExecutionMode untouched so an invalid mode still fails validation.
//   - Axes set: treat them as authoritative and (re)derive the legacy fields the
//     executor layer still reads, so downstream dispatch is unchanged.
func normalizeExecutionAxes(in *Input) {
	if in.Engine == "" && in.Destination == "" {
		in.Engine, in.Destination, in.Orchestrator = axesFromLegacy(in.ExecutionMode, in.Scheduler)

		return
	}

	if in.Engine == "" {
		in.Engine = EngineNONMEM
	}

	if in.Destination == "" {
		in.Destination = DestinationHere
	}

	if in.Destination == DestinationHermes && in.Orchestrator == "" {
		in.Orchestrator = OrchestratorDocker
	}

	in.ExecutionMode = DeriveExecutionMode(in.Engine, in.Destination)

	// Only the Scheduler destination carries a real scheduler; Here and Hermes
	// run locally.
	if in.Destination != DestinationScheduler {
		in.Scheduler = "LOCAL"
	}
}

// ValidateSLURMMode validates the SLURM mode configuration.
func ValidateSLURMMode(config SLURMConfig) error {
	if config.Mode == "" {
		// Default to CLI mode if not specified
		return nil
	}

	switch config.Mode {
	case SLURMModeREST:
		// REST mode validation
		if config.REST.SocketPath == "" {
			return fmt.Errorf("SLURM REST mode requires socket_path to be configured")
		}
		if config.REST.APIVersion == "" {
			return fmt.Errorf("SLURM REST mode requires api_version to be configured")
		}

		return nil

	case SLURMModeCLI:
		// CLI mode validation - no specific requirements for now
		return nil

	default:
		return fmt.Errorf("invalid SLURM mode: %q (must be one of: %s, %s)",
			config.Mode, SLURMModeREST, SLURMModeCLI)
	}
}

// GetValidSLURMModes returns a list of valid SLURM modes.
func GetValidSLURMModes() []string {
	return []string{SLURMModeREST, SLURMModeCLI}
}

// ValidateNONMEMConfig validates the NONMEM configuration structure.
// This does NOT validate that the license file exists - that validation is
// deferred to model load/execution time to allow the application to start
// without requiring a NONMEM license. The license will be checked when
// a model is loaded and the user attempts to execute it.
func ValidateNONMEMConfig(_ NONMEMConfig) error {
	// No validation at startup - license existence is checked at model load/execution time
	// This allows Janus to support multiple modeling engines without requiring
	// all licenses to be present at startup
	return nil
}

// ExpandNONMEMLicensePath expands the home directory in the NONMEM license path.
// Returns the expanded path or empty string if not configured.
func ExpandNONMEMLicensePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}

		return filepath.Join(home, path[1:]), nil
	}

	return path, nil
}

// ValidateNONMEMLicenseForExecution validates that a NONMEM license file exists
// and is accessible. This should be called at execution time, not at startup.
//
// The function checks the following locations in order:
//  1. The configured path in nonmem.license.path (if specified)
//  2. ~/nonmem.lic (default location)
//  3. ./nonmem.lic (current directory fallback)
//
// Returns the validated license path or an error if no valid license is found.
func ValidateNONMEMLicenseForExecution(cfg *Config) (string, error) {
	// Check configured path first
	if cfg.NONMEM.License.Path != "" {
		expandedPath, err := ExpandNONMEMLicensePath(cfg.NONMEM.License.Path)
		if err != nil {
			return "", fmt.Errorf("failed to expand license path: %w", err)
		}

		if _, err := os.Stat(expandedPath); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("NONMEM license file not found at configured path: %s", expandedPath)
			}

			return "", fmt.Errorf("cannot access NONMEM license file at %s: %w", expandedPath, err)
		}

		return expandedPath, nil
	}

	// Try default locations
	home, err := os.UserHomeDir()
	if err == nil {
		homeLicense := filepath.Join(home, "nonmem.lic")
		if _, err := os.Stat(homeLicense); err == nil {
			return homeLicense, nil
		}
	}

	// Try current directory
	if _, err := os.Stat("./nonmem.lic"); err == nil {
		return "./nonmem.lic", nil
	}

	return "", fmt.Errorf("NONMEM license file not found. Please configure 'nonmem.license.path' in your config file, or place nonmem.lic in your home directory or current working directory")
}

// RequiresNONMEMLicense returns true if the execution mode requires a NONMEM license.
func RequiresNONMEMLicense(mode string) bool {
	switch mode {
	case ExecutionModeNONMEM, ExecutionModeHERMES, ExecutionModeBBI, ExecutionModePSN:
		return true
	default:
		return false
	}
}

// ExpandSigningPrivateKeyPath expands the home directory in the signing private key path.
// Returns the expanded path or empty string if not configured.
func ExpandSigningPrivateKeyPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}

		return filepath.Join(home, path[1:]), nil
	}

	return path, nil
}

// Signing key backends. See SigningConfig.Backend.
const (
	// SigningBackendKeychain keeps the private key in the OS credential store:
	// Keychain on macOS, Credential Manager on Windows, Secret Service on Linux.
	SigningBackendKeychain = "keychain"

	// SigningBackendFile keeps the private key in a PEM file on disk.
	SigningBackendFile = "file"
)

// ErrUnknownSigningBackend reports a signing.backend value that names neither
// supported backend.
var ErrUnknownSigningBackend = errors.New("unknown signing backend")

// ResolveSigningBackend reports which backend cfg selects, and whether signing is
// configured at all. An explicit Backend wins; otherwise a PrivateKeyPath means
// file and an Identity alone means keychain.
//
// An unrecognised Backend is an error rather than something to infer past. A
// typo such as "keyring" for "keychain" would otherwise fall through to the
// inference below and quietly select the *other* backend — signing with a stale
// key file instead of the credential-store key the user asked for, under a
// different fingerprint, with no diagnostic.
func ResolveSigningBackend(cfg SigningConfig) (backend string, enabled bool, err error) {
	switch cfg.Backend {
	case SigningBackendKeychain:
		return SigningBackendKeychain, cfg.Identity != "", nil
	case SigningBackendFile:
		return SigningBackendFile, cfg.PrivateKeyPath != "", nil
	case "":
		// Nothing declared — infer from what else is configured.
	default:
		return "", false, fmt.Errorf("%w %q (want %q or %q)",
			ErrUnknownSigningBackend, cfg.Backend, SigningBackendKeychain, SigningBackendFile)
	}

	if cfg.PrivateKeyPath != "" {
		return SigningBackendFile, true, nil
	}

	if cfg.Identity != "" {
		return SigningBackendKeychain, true, nil
	}

	return "", false, nil
}

// ExpandSigningKeyringPath expands the home directory in the signing keyring path.
// Returns the expanded path or empty string if not configured.
func ExpandSigningKeyringPath(path string) (string, error) {
	return ExpandSigningPrivateKeyPath(path)
}

// ValidateSigningConfig validates the signing configuration.
// This checks that if a private key path is specified, the file exists.
// Returns nil if signing is not configured (optional feature).
func ValidateSigningConfig(cfg SigningConfig) error {
	if cfg.PrivateKeyPath == "" {
		// Signing is optional - no configuration means no signing
		return nil
	}

	expandedPath, err := ExpandSigningPrivateKeyPath(cfg.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand signing private key path: %w", err)
	}

	if _, err := os.Stat(expandedPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("signing private key file not found: %s", expandedPath)
		}

		return fmt.Errorf("cannot access signing private key file at %s: %w", expandedPath, err)
	}

	return nil
}

// ValidateHermesConfig validates the Hermes global configuration.
// Note: hermes.image is model-specific and validated at execution time, not here.
// License validation has moved to ValidateNONMEMConfig and the category handler.
func ValidateHermesConfig(config HermesConfig) error {
	// Image is model-specific (in .janus.config.json), not required in global config
	// Skip image validation here

	// License validation is now handled by:
	// 1. ValidateNONMEMConfig (if nonmem.license.path is specified)
	// 2. Category handler fallbacks (~/nonmem.lic, ./nonmem.lic)
	// The hermes.license.path is deprecated and migrated to nonmem.license.path in NewConfig

	// Validate container port (if specified)
	if config.Container.Port != 0 && (config.Container.Port < 1024 || config.Container.Port > 65535) {
		return fmt.Errorf("invalid Hermes container port: %d (must be between 1024 and 65535)", config.Container.Port)
	}

	return nil
}

// ValidateHermesKubernetes enforces the Kubernetes-orchestrator requirements: a
// target namespace must be configured so Janus knows where to provision the
// Hermes pod. It is a no-op unless the Kubernetes orchestrator is selected.
func ValidateHermesKubernetes(orchestrator string, k8s HermesKubernetesConfig) error {
	if orchestrator != OrchestratorKubernetes {
		return nil
	}

	if strings.TrimSpace(k8s.Namespace) == "" {
		return fmt.Errorf("kubernetes orchestrator requires hermes.kubernetes.namespace to be set")
	}

	if k8s.BootstrapParallelism < 0 {
		return fmt.Errorf("hermes.kubernetes.bootstrap_parallelism must not be negative, got %d", k8s.BootstrapParallelism)
	}

	if _, err := k8s.FitTimeoutDuration(); err != nil {
		return err
	}

	return nil
}

// MCP server defaults.
const (
	// DefaultMCPHost is the default (loopback) bind address for the MCP server.
	DefaultMCPHost = "127.0.0.1"

	// DefaultMCPPort is the default TCP port for the MCP server.
	DefaultMCPPort = 8731
)

// isLoopbackHost reports whether host is a permitted loopback bind address.
func isLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// ValidateMCPConfig validates and normalizes the embedded MCP server config.
//
// It is a no-op when the server is disabled. When enabled it defaults the host
// and port, enforces a loopback-only bind address (the server is never exposed
// off the local machine), and validates the port range. It mutates cfg in place
// so the normalized host/port are used by callers.
func ValidateMCPConfig(cfg *MCPConfig) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	if cfg.Host == "" {
		cfg.Host = DefaultMCPHost
	}

	if !isLoopbackHost(cfg.Host) {
		return fmt.Errorf("invalid MCP host: %q (must be loopback: 127.0.0.1, localhost, or ::1)", cfg.Host)
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultMCPPort
	}

	if cfg.Port < 1024 || cfg.Port > 65535 {
		return fmt.Errorf("invalid MCP port: %d (must be between 1024 and 65535)", cfg.Port)
	}

	return nil
}

// InitializerOptions holds configuration for the configuration initializer.
type InitializerOptions struct {
	ConfigFlagName    string // The name of the config flag to read
	DefaultConfigPath string // Default config file path
	SuppressOutput    bool   // Whether to suppress stdout/stderr output
}

// NewInitializer creates a PreRunE function that initializes viper configuration
// and unmarshals it onto the provided config pointer.
// This allows any cobra command to easily set up configuration.
func NewInitializer(configPtr **Config, opts InitializerOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get config file from flag
		var configFile string
		if opts.ConfigFlagName != "" {
			flagValue, err := cmd.Flags().GetString(opts.ConfigFlagName)
			if err == nil {
				configFile = flagValue
			}
		}

		// Determine config file path
		configPath := opts.DefaultConfigPath
		if configPath == "" {
			configPath = getDefaultConfigPath()
		}

		if configFile != "" {
			// Use config file from the flag
			viper.SetConfigFile(configFile)
		} else {
			// Use default config path
			viper.SetConfigFile(configPath)
		}

		// Try to read config file
		if err := viper.ReadInConfig(); err != nil {
			// Check if config file doesn't exist (handle both viper and filesystem errors)
			var configFileNotFoundErr viper.ConfigFileNotFoundError
			if errors.As(err, &configFileNotFoundErr) || os.IsNotExist(err) {
				// Config file not found - this will be handled by the GUI command
				// which will show the setup wizard. For non-GUI commands, we'll
				// need basic defaults to work with.
				if !opts.SuppressOutput {
					fmt.Fprintln(os.Stderr, "No config file found. Run 'janus gui' to set up initial configuration.")
				}

				return nil
			} else {
				// Config file was found but another error occurred
				if !opts.SuppressOutput {
					fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
				}

				return err
			}
		} else if !opts.SuppressOutput {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}

		// Process configuration and assign to the pointer
		cfg, err := Process()
		if err != nil {
			return fmt.Errorf("failed to process configuration: %w", err)
		}

		*configPtr = cfg

		return nil
	}
}

// getDefaultConfigPath returns the default configuration file path.
func getDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return "./config.yml"
	}

	return filepath.Join(home, ".config", "janus", "config.yml")
}

// ErrNoConfigFile reports that there is no config file to update. Callers decide
// what to do about it; the signing commands tell the user which two lines to add
// rather than failing, because the key itself is already safely stored.
var ErrNoConfigFile = errors.New("no config file to update")

// persistSigningKeys updates an existing config file with the given settings.
//
// It deliberately refuses to create one. viper.WriteConfig serialises viper's
// whole in-memory state, which on a fresh install is nothing but flag defaults —
// producing a file with no execution_mode that Janus then cannot load. Better to
// leave the user without a config file than with a broken one.
func persistSigningKeys(settings map[string]string) error {
	path := viper.ConfigFileUsed()
	if path == "" {
		return ErrNoConfigFile
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNoConfigFile
		}

		return fmt.Errorf("checking config file %s: %w", path, err)
	}

	for key, value := range settings {
		viper.Set(key, value)
	}

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// PersistSigningIdentity records the signing backend and identity in the user's
// config file, so a key created by `janus keys generate` is picked up on the next
// start without hand-editing YAML. It returns ErrNoConfigFile when there is no
// config file to update.
func PersistSigningIdentity(backend, identity string) error {
	return persistSigningKeys(map[string]string{
		"signing.backend":  backend,
		"signing.identity": identity,
	})
}

// PersistSigningKeyringPath records the trust keyring location. Signing and
// verification are independent, so this is separate from PersistSigningIdentity:
// configuring one must not silently configure the other.
func PersistSigningKeyringPath(path string) error {
	return persistSigningKeys(map[string]string{"signing.keyring_path": path})
}
