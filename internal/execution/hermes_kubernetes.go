package execution

import (
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is used only to derive a short, non-cryptographic pod name/label
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/kube"
	"github.com/pharmalytica/janus/internal/runlog"
)

// hermesEndpoint is what an orchestrator (Docker container or Kubernetes pod)
// hands the gRPC transport: where to dial, an identifier for provenance, and the
// resolved image metadata. It is the seam that lets executeViaGRPC stay
// orchestrator-agnostic.
type hermesEndpoint struct {
	addr      string                 // host:port the Hermes gRPC client dials
	id        string                 // full identifier (Docker container ID / pod name) — audit trail
	displayID string                 // short, human-meaningful identifier — run record
	image     *runlog.ContainerImage // resolved image provenance
}

// hermesPodApp is the value of the "app" label on Janus-managed Hermes pods.
const hermesPodApp = "janus-hermes"

// defaultHermesPort is the Hermes gRPC port inside the container/pod when none is
// configured (matches the Docker path default).
const defaultHermesPort = 50051

// defaultPodReadyTimeout is how long to wait for the Hermes pod to become Ready
// (image pull + scheduling can take a while), used when no startup timeout is
// configured.
const defaultPodReadyTimeout = 5 * time.Minute

// usesKubernetes reports whether the configured orchestrator is Kubernetes.
func (e *HermesExecutor) usesKubernetes() bool {
	return e.config != nil && e.config.Orchestrator == config.OrchestratorKubernetes
}

// hermesOverrideConfigYAML renders the Hermes server config that maps the logical
// "nonmem" command to the model's container command path. It is shared by the
// Docker path (written to a file and bind-mounted) and the Kubernetes path
// (stored in a ConfigMap and mounted), so both deliver the same command mapping
// Hermes reads via HERMES_CONFIG.
func hermesOverrideConfigYAML(commandPath string) string {
	return fmt.Sprintf(`server:
  address: "0.0.0.0"
  port: 50051

executor:
  mode: "local"
  local:
    workspace_base: ""

overrides:
  commands:
    - pattern: "nonmem"
      target: %q
      description: "Modeling tool executable from Janus config"
    - pattern: "*/nonmem"
      target: %q
      description: "Modeling tool executable with path prefix"
`, commandPath, commandPath)
}

// SweepOrphanedHermesPods best-effort removes every Janus-managed Hermes pod (and
// its config map) left in the configured Kubernetes namespace — orphans from a
// run whose process was killed before its teardown/sweep could run. It is the
// startup safety net behind the per-run pre-flight sweep and the saga's deferred
// sweep.
//
// It is gated on a configured namespace (a no-op otherwise). It removes ALL
// app=janus-hermes pods in that namespace, so it assumes the namespace belongs to
// this host — the host-orchestrated model where each user drives their own
// namespace with their own credentials. In a namespace shared by concurrent Janus
// instances it could remove another instance's in-flight pods.
func SweepOrphanedHermesPods(ctx context.Context, cfg *config.Config) error {
	if cfg == nil || strings.TrimSpace(cfg.Hermes.Kubernetes.Namespace) == "" {
		return nil
	}

	k8sCfg := cfg.Hermes.Kubernetes

	clientset, err := kube.NewClientset(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return fmt.Errorf("connecting to kubernetes: %w", err)
	}

	restConfig, err := kube.RESTConfig(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return fmt.Errorf("loading kubernetes config: %w", err)
	}

	orchestrator := kube.NewOrchestrator(clientset, restConfig, k8sCfg.Namespace)

	return orchestrator.DeleteStale(ctx, fmt.Sprintf("app=%s", hermesPodApp))
}

// executeOnKubernetes runs the model by provisioning a Hermes pod, port-forwarding
// it to localhost, and reusing the shared gRPC transport. It mirrors the Docker
// path's pre-flight cleanup, execution, post-hooks, and teardown.
func (e *HermesExecutor) executeOnKubernetes(ctx context.Context, command string, args []string, licenseData []byte, modelFiles map[string][]byte, modelPath, jobID string, startTime time.Time) (*ExecutionResult, error) {
	k8sCfg := e.config.Hermes.Kubernetes

	clientset, err := kube.NewClientset(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return nil, fmt.Errorf("connecting to kubernetes: %w", err)
	}

	restConfig, err := kube.RESTConfig(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return nil, fmt.Errorf("loading kubernetes config: %w", err)
	}

	orchestrator := kube.NewOrchestrator(clientset, restConfig, k8sCfg.Namespace)

	port := e.config.Hermes.Container.Port
	if port == 0 {
		port = defaultHermesPort
	}

	modelHash := shortModelHash(modelPath)
	labels := map[string]string{"app": hermesPodApp, "janus-model": modelHash}
	selector := fmt.Sprintf("app=%s,janus-model=%s", hermesPodApp, modelHash)

	// Pre-flight cleanup: clear any orphaned pod from a prior, interrupted run of
	// this exact model (best-effort, like the Docker stale-container cleanup).
	if derr := orchestrator.DeleteStale(ctx, selector); derr != nil {
		log.Printf("Warning: failed to clear stale Hermes pods: %v", derr)
	}

	spec := kube.PodSpec{
		Name:            k8sPodName(modelPath, modelHash),
		Image:           e.modelConfig.Image,
		Port:            port,
		CPUCores:        e.modelConfig.Resources.CPUCores,
		Memory:          e.modelConfig.Resources.Memory,
		Labels:          labels,
		ImagePullSecret: k8sCfg.ImagePullSecret,
	}

	// Map the logical "nonmem" command to the model's container command path,
	// delivered via a mounted ConfigMap (mirrors the Docker bind-mount).
	if commandPath := e.modelConfig.GetCommandPath(); commandPath != "" {
		spec.HermesConfigYAML = hermesOverrideConfigYAML(commandPath)
	}

	workload, err := orchestrator.Provision(ctx, spec, e.podReadyTimeout())
	if err != nil {
		return nil, fmt.Errorf("provisioning Hermes pod: %w", err)
	}

	// Always tear the pod down (and stop the port-forward) when we're done —
	// issue #13 step 8.
	defer workload.Teardown()

	name, tag := e.parseImageReference(e.modelConfig.Image)
	endpoint := hermesEndpoint{
		addr:      workload.LocalAddr,
		id:        workload.PodName, // namespace+pod identifies the workload for audit
		displayID: workload.PodName,
		image: &runlog.ContainerImage{
			Name:   name,
			Tag:    tag,
			Digest: imageDigest(workload.ImageID), // normalize the pod's resolved image ref to a bare digest
			Full:   e.modelConfig.Image,
		},
	}

	result, err := e.executeViaGRPC(ctx, command, args, licenseData, modelFiles, endpoint, modelPath, jobID, startTime)
	if err != nil {
		// Surface the pod's own logs so the failure is debuggable from the app
		// (the pod is torn down by the deferred Teardown right after this).
		if logs, logErr := orchestrator.PodLogs(ctx, workload.PodName, 50); logErr == nil && strings.TrimSpace(logs) != "" {
			return nil, fmt.Errorf("execution via gRPC failed: %w\n--- pod %s logs (tail) ---\n%s", err, workload.PodName, logs)
		}

		return nil, fmt.Errorf("execution via gRPC failed: %w", err)
	}

	// Run post-run integration hooks (e.g. R diagnostics). No-op by default.
	applyPostHooks(ctx, e.config, modelPath)

	return result, nil
}

// podReadyTimeout returns the pod-ready wait timeout: the configured Hermes
// startup timeout if set, otherwise defaultPodReadyTimeout.
func (e *HermesExecutor) podReadyTimeout() time.Duration {
	if e.config != nil && e.config.Hermes.Container.StartupTimeout != "" {
		if d, err := time.ParseDuration(e.config.Hermes.Container.StartupTimeout); err == nil {
			return d
		}
	}

	return defaultPodReadyTimeout
}

// imageDigest extracts the bare image digest (e.g. "sha256:abc…") from a
// Kubernetes pod's resolved imageID, which may carry a scheme and/or repository
// prefix (e.g. "docker-pullable://ghcr.io/org/img@sha256:abc…"). This matches
// the digest format the Docker path records. Returns the input unchanged when no
// digest is present.
func imageDigest(imageID string) string {
	if at := strings.Index(imageID, "@"); at >= 0 {
		return imageID[at+1:]
	}

	if sha := strings.Index(imageID, "sha256:"); sha >= 0 {
		return imageID[sha:]
	}

	return imageID
}

// safeShortID truncates an identifier to 12 characters for display, without
// panicking on short identifiers (a pod name may be shorter than a Docker ID).
func safeShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}

	return id
}

// shortModelHash returns a short, stable, label-safe hash of a model path, used
// to tie a pod to its model for selection and cleanup.
func shortModelHash(modelPath string) string {
	sum := sha1.Sum([]byte(modelPath)) //nolint:gosec // non-cryptographic identifier

	return fmt.Sprintf("%x", sum)[:10]
}

// k8sPodName builds a DNS-1123-compliant pod name from the model file name and a
// short hash, so it is stable per model and unique across models.
func k8sPodName(modelPath, hash string) string {
	base := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
	name := fmt.Sprintf("%s-%s-%s", hermesPodApp, sanitizeDNS1123(base), hash)

	// Pod names are limited to 253 chars; stay well under for readability.
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}

	return name
}

// sanitizeDNS1123 lowercases s and replaces any character outside [a-z0-9-] with
// '-', trimming leading/trailing dashes, so the result is a valid DNS-1123 label
// segment. Falls back to "model" when nothing usable remains.
func sanitizeDNS1123(s string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "model"
	}

	return out
}
