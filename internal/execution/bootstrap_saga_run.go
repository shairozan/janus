package execution

import (
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution/category"
	"github.com/shairozan/janus/internal/kube"
	"github.com/shairozan/janus/internal/runlog"
)

// stagePod is a request to run one Hermes pod stage of the bootstrap saga.
type stagePod struct {
	image      string      // container image (PsN orchestration image, or NONMEM execution image)
	configYAML string      // optional Hermes command-mapping ConfigMap content (fits need "nonmem"->nmfe)
	namePrefix string      // pod-name prefix (e.g. "bs-setup", "bs-fit-3", "bs-aggregate")
	stageLabel string      // janus-stage label value, for selection/cleanup
	sagaID     string      // per-run id, for unique pod names + the janus-saga label
	stage      hermesStage // the gRPC request + where returned files land
}

// stageRunner provisions a pod for p, runs the stage over gRPC, collects its
// returned files, and tears the pod down. It is the seam that keeps the saga
// orchestration (sequence, fan-out, merging) unit-testable without a cluster.
type stageRunner interface {
	runStage(ctx context.Context, p stagePod) (*stageOutcome, error)

	// cleanup best-effort removes every pod and config map this saga created,
	// matched by the per-run janus-saga label. It detaches from ctx's cancellation
	// so it still runs when the saga is cancelled, and is safe to call repeatedly
	// (it deletes only what currently exists). The saga calls it once before
	// starting (to clear orphans from a prior interrupted run, whose pod names —
	// derived from the stable saga id — would otherwise collide) and once on exit
	// (the safety net behind the per-stage teardowns).
	cleanup(ctx context.Context, sagaID string) error
}

// BootstrapSagaResult summarizes a completed bootstrap saga for the caller (and,
// later, the run log).
type BootstrapSagaResult struct {
	Samples     int            // resampled datasets produced by SETUP
	Succeeded   int            // fits that returned a list file
	Failed      int            // fits that errored or returned nothing
	OutputFiles []string       // result files written to the model dir (raw_results, bootstrap_results)
	FitErrors   map[int]string // per-failed-fit error, for diagnostics
}

// IsBootstrapSagaRun reports whether the selected axes + PsN function call for
// the horizontal bootstrap saga: PsN engine, bootstrap function, Hermes
// destination, Kubernetes orchestrator. The GUI/command dispatch uses this to
// route to the saga instead of the local PsN executor.
func IsBootstrapSagaRun(cfg *config.Config, psnFunction string) bool {
	return cfg != nil &&
		cfg.Engine == config.EnginePSN &&
		cfg.Destination == config.DestinationHermes &&
		cfg.Orchestrator == config.OrchestratorKubernetes &&
		psnFunction == "bootstrap"
}

// BootstrapSaga runs PsN bootstrap as a host-orchestrated, horizontal fan-out on
// Kubernetes (#192): a PsN-image SETUP pod resamples, a FIFO pool of
// execution-image fit pods runs the fits, and a PsN-image AGGREGATE pod computes
// the confidence intervals. The desktop is the data hub between stages (no shared
// PVC); every pod is created, driven, and torn down by the host.
type BootstrapSaga struct {
	runner      stageRunner
	psnImage    string
	execImage   string
	commandPath string // container nmfe path, for the fit pods' command mapping
	cpuCores    int
	memory      string
	parallelism int
	fitTimeout  time.Duration

	// collectFiles gathers the SETUP input workspace (model + data) for a model
	// path; getLicense returns the NONMEM license bytes for the fit pods. Both are
	// injectable so the orchestration is testable; the real constructor wires them
	// to the NONMEM category.
	collectFiles func(modelPath string) (map[string][]byte, error)
	getLicense   func() ([]byte, error)

	// Live-progress state (see bootstrap_saga_progress.go). Guarded by progressMu;
	// zero-valued (no callback, nil map) when nobody is watching, so the saga runs
	// unchanged without a reporter.
	progressMu sync.Mutex
	progressFn func(SagaProgress)
	prog       SagaProgress
	livePods   map[string]SagaPod
}

// NewBootstrapSaga builds a saga that orchestrates the bootstrap on the cluster
// named by the global Hermes Kubernetes config, using the model's execution image
// + resources for the fits and the resolved PsN image for setup/aggregate.
func NewBootstrapSaga(cfg *config.Config, modelConfig *config.HermesExecutionConfig) (*BootstrapSaga, error) {
	if cfg == nil || modelConfig == nil {
		return nil, fmt.Errorf("bootstrap saga requires non-nil config and model config")
	}

	psnImage, err := modelConfig.PsNImageOrDefault(cfg.Hermes.PsNImage)
	if err != nil {
		return nil, err
	}

	k8sCfg := cfg.Hermes.Kubernetes

	clientset, err := kube.NewClientset(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return nil, fmt.Errorf("connecting to kubernetes: %w", err)
	}

	restConfig, err := kube.RESTConfig(k8sCfg.Kubeconfig, k8sCfg.Context)
	if err != nil {
		return nil, fmt.Errorf("loading kubernetes config: %w", err)
	}

	port := cfg.Hermes.Container.Port
	if port == 0 {
		port = defaultHermesPort
	}

	fitTimeout, err := k8sCfg.FitTimeoutDuration()
	if err != nil {
		return nil, err
	}

	runner := &kubeStageRunner{
		orchestrator: kube.NewOrchestrator(clientset, restConfig, k8sCfg.Namespace),
		port:         port,
		pullSecret:   k8sCfg.ImagePullSecret,
		readyTimeout: hermesPodReadyTimeout(cfg),
	}

	nonmemCategory := category.NewNONMEMCategory()

	return &BootstrapSaga{
		runner:      runner,
		psnImage:    psnImage,
		execImage:   modelConfig.Image,
		commandPath: modelConfig.GetCommandPath(),
		cpuCores:    modelConfig.Resources.CPUCores,
		memory:      modelConfig.Resources.Memory,
		parallelism: k8sCfg.BootstrapParallelismOrDefault(),
		fitTimeout:  fitTimeout,
		collectFiles: func(modelPath string) (map[string][]byte, error) {
			return nonmemCategory.ContainerStructure(modelPath, cfg)
		},
		getLicense: func() ([]byte, error) {
			return nonmemCategory.GetLicense(cfg)
		},
	}, nil
}

// Run executes the three-stage saga for the model at modelPath with the given
// number of bootstrap samples, writing the result files (raw_results,
// bootstrap_results with CIs) into the model directory.
func (s *BootstrapSaga) Run(ctx context.Context, modelPath string, samples int) (*BootstrapSagaResult, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("bootstrap samples must be positive, got %d", samples)
	}

	modelFile := filepath.Base(modelPath)
	modelBase := strings.TrimSuffix(modelFile, filepath.Ext(modelFile))
	sagaID := shortModelHash(modelPath)

	// Clear any pods orphaned by a previous interrupted run of this model: the pod
	// names derive from the stable saga id, so leftovers would otherwise collide
	// with this run's pods ("already exists").
	if err := s.runner.cleanup(ctx, sagaID); err != nil {
		log.Printf("bootstrap saga %s: pre-run pod sweep failed (continuing): %v", sagaID, err)
	}

	// Safety net behind the per-stage teardowns: on any exit — success, error, or
	// cancellation — sweep every pod this saga created by label, so a teardown that
	// raced or silently failed can't leak pods. cleanup detaches from ctx, so it
	// runs even after cancellation.
	defer func() {
		if cerr := s.runner.cleanup(ctx, sagaID); cerr != nil {
			log.Printf("bootstrap saga %s: final pod sweep failed: %v", sagaID, cerr)
		}
	}()

	// ---- Stage 1: SETUP (PsN image) — resample only, no fits. ----------------
	setupFiles, err := s.collectFiles(modelPath)
	if err != nil {
		return nil, fmt.Errorf("collecting bootstrap input files: %w", err)
	}

	s.setStage(SagaStageSetup, 0)
	setupPod := sagaPodName(bsSetupPodPrefix, sagaID)
	s.podStarted(setupPod, 0, time.Now())

	setupOut, err := s.runner.runStage(ctx, stagePod{
		image:      s.psnImage,
		namePrefix: bsSetupPodPrefix,
		stageLabel: "setup",
		sagaID:     sagaID,
		stage: hermesStage{
			command:  "bash",
			args:     []string{"-lc", bootstrapResampleScript(modelFile, samples)},
			files:    setupFiles,
			retain:   setupRetain(),
			cpuCores: s.cpuCores,
			memory:   s.memory,
		},
	})
	s.podFinished(setupPod, false, err == nil)
	if err != nil {
		return nil, fmt.Errorf("bootstrap setup stage: %w", err)
	}

	// Judge SETUP by produced files, not exit code (PsN exits non-zero via the
	// dummy compiler) — see #193.
	indices := resampledIndices(setupOut.files)
	if len(indices) == 0 {
		return nil, fmt.Errorf("bootstrap setup produced no resampled datasets (requested %d samples)", samples)
	}

	// ---- Stage 2: FITS (execution image) — FIFO sliding-window pool. ----------
	license, err := s.getLicense()
	if err != nil {
		return nil, fmt.Errorf("loading NONMEM license for fit pods: %w", err)
	}

	fitConfigYAML := ""
	if s.commandPath != "" {
		fitConfigYAML = hermesOverrideConfigYAML(s.commandPath)
	}

	s.setStage(SagaStageFits, len(indices))

	fits := fanOut(ctx, indices, s.parallelism, func(ctx context.Context, _ int, i int) (*stageOutcome, error) {
		podName := sagaPodName(fmt.Sprintf(bsFitPodPrefixFmt, i), sagaID)
		s.podStarted(podName, i, time.Now())

		files, ok := fitInputs(setupOut.files, i)
		if !ok {
			s.podFinished(podName, true, false)

			return nil, fmt.Errorf("missing inputs for resample %d", i)
		}

		cmd, args := fitArgs(i)

		out, err := s.runner.runStage(ctx, stagePod{
			image:      s.execImage,
			configYAML: fitConfigYAML,
			namePrefix: fmt.Sprintf(bsFitPodPrefixFmt, i),
			stageLabel: "fit",
			sagaID:     sagaID,
			stage: hermesStage{
				command:  cmd,
				args:     args,
				license:  license,
				files:    files,
				retain:   fitRetain(i),
				cpuCores: s.cpuCores,
				memory:   s.memory,
				timeout:  s.fitTimeout,
			},
		})
		s.podFinished(podName, true, err == nil)

		return out, err
	})

	workspace, succeeded, fitErrors := mergeBootstrapWorkspace(setupOut.files, indices, fits)
	if succeeded == 0 {
		return nil, fmt.Errorf("no bootstrap fits succeeded (%d failed); cannot aggregate", len(fitErrors))
	}

	// Host-as-data-hub: persist the full fit workspace (resampled datasets + every
	// successful fit's outputs) under the model directory before aggregating, so
	// the completed saga is auditable and its per-fit artifacts are downloadable.
	// The AGGREGATE stage then writes the result CSVs into the same bs/ tree,
	// overwriting any stale raw_results carried over from SETUP.
	writeCollectedFiles(filepath.Dir(modelPath), workspace)

	// ---- Stage 3: AGGREGATE (PsN image) — rawresults + -summarize -> CIs. -----
	s.setStage(SagaStageAggregate, 0)
	aggPod := sagaPodName(bsAggregatePodPrefix, sagaID)
	s.podStarted(aggPod, 0, time.Now())

	aggOut, err := s.runner.runStage(ctx, stagePod{
		image:      s.psnImage,
		namePrefix: bsAggregatePodPrefix,
		stageLabel: "aggregate",
		sagaID:     sagaID,
		stage: hermesStage{
			command:   "bash",
			args:      []string{"-lc", bootstrapAggregateScript(modelFile, modelBase)},
			files:     workspace,
			retain:    aggregateRetain(),
			outputDir: filepath.Dir(modelPath),
			cpuCores:  s.cpuCores,
			memory:    s.memory,
		},
	})
	s.podFinished(aggPod, false, err == nil)
	if err != nil {
		return nil, fmt.Errorf("bootstrap aggregate stage: %w", err)
	}

	return &BootstrapSagaResult{
		Samples:     len(indices),
		Succeeded:   succeeded,
		Failed:      len(fitErrors),
		OutputFiles: aggOut.outputPaths,
		FitErrors:   fitErrors,
	}, nil
}

// mergeBootstrapWorkspace rebuilds the bootstrap workspace for the AGGREGATE
// stage: the SETUP-returned tree plus each successful fit's list/ext files,
// re-keyed under <ws>/m1. It returns the merged files, the count of successful
// fits, and a map of failed sample index -> error message.
func mergeBootstrapWorkspace(setupFiles map[string][]byte, indices []int, fits []fanResult[*stageOutcome]) (workspace map[string][]byte, succeeded int, fitErrors map[int]string) {
	workspace = make(map[string][]byte, len(setupFiles)+2*len(indices))
	maps.Copy(workspace, setupFiles)
	fitErrors = make(map[int]string)

	for pos, i := range indices {
		res := fits[pos]

		if res.Err != nil || res.Value == nil {
			msg := "no result"
			if res.Err != nil {
				msg = res.Err.Error()
			}

			fitErrors[i] = msg

			continue
		}

		lst, hasLst := res.Value.files[fitListName(i)]
		ext, hasExt := res.Value.files[fitExtName(i)]

		if !hasLst {
			fitErrors[i] = "fit produced no list file"

			continue
		}

		workspace[m1Path(fitListName(i))] = lst
		if hasExt {
			workspace[m1Path(fitExtName(i))] = ext
		}

		succeeded++
	}

	return workspace, succeeded, fitErrors
}

// kubeStageRunner is the cluster-backed stageRunner: each stage gets a freshly
// provisioned, port-forwarded pod, driven over the shared gRPC transport and
// torn down afterward.
type kubeStageRunner struct {
	orchestrator   *kube.Orchestrator
	port           int
	pullSecret     string
	readyTimeout   time.Duration
	stdout, stderr io.Writer
}

func (r *kubeStageRunner) runStage(ctx context.Context, p stagePod) (*stageOutcome, error) {
	name := sagaPodName(p.namePrefix, p.sagaID)

	spec := kube.PodSpec{
		Name:             name,
		Image:            p.image,
		Port:             r.port,
		CPUCores:         p.stage.cpuCores,
		Memory:           p.stage.memory,
		Labels:           map[string]string{"app": hermesPodApp, "janus-saga": p.sagaID, "janus-stage": p.stageLabel},
		ImagePullSecret:  r.pullSecret,
		HermesConfigYAML: p.configYAML,
	}

	workload, err := r.orchestrator.Provision(ctx, spec, r.readyTimeout)
	if err != nil {
		return nil, fmt.Errorf("provisioning %s pod: %w", p.stageLabel, err)
	}
	defer workload.Teardown()

	ep := hermesEndpoint{
		addr:      workload.LocalAddr,
		id:        workload.PodName,
		displayID: workload.PodName,
		image:     &runlog.ContainerImage{Full: p.image, Digest: imageDigest(workload.ImageID)},
	}

	out, err := streamHermesExecution(ctx, ep, p.stage, r.stdout, r.stderr)
	if err != nil {
		if logs, lerr := r.orchestrator.PodLogs(ctx, workload.PodName, 50); lerr == nil && strings.TrimSpace(logs) != "" {
			return nil, fmt.Errorf("%s stage failed: %w\n--- pod %s logs (tail) ---\n%s", p.stageLabel, err, workload.PodName, logs)
		}

		return nil, fmt.Errorf("%s stage failed: %w", p.stageLabel, err)
	}

	return out, nil
}

// cleanup deletes every pod and config map labelled with this saga's id, using a
// context detached from ctx's cancellation so the sweep still runs when the saga
// is cancelled.
func (r *kubeStageRunner) cleanup(ctx context.Context, sagaID string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	return r.orchestrator.DeleteStale(ctx, fmt.Sprintf("janus-saga=%s", sagaID))
}

// sagaPodName builds a DNS-1123 pod name from a stage prefix and a per-run
// suffix, bounded to 63 characters.
func sagaPodName(prefix, suffix string) string {
	name := sanitizeDNS1123(prefix)
	if suffix != "" {
		name += "-" + suffix
	}

	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}

	return name
}

// hermesPodReadyTimeout resolves the pod-ready wait from the configured Hermes
// startup timeout, falling back to the default.
func hermesPodReadyTimeout(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.Hermes.Container.StartupTimeout != "" {
		if d, err := time.ParseDuration(cfg.Hermes.Container.StartupTimeout); err == nil {
			return d
		}
	}

	return defaultPodReadyTimeout
}
