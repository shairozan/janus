package execution

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/config"
)

func TestImageDigest(t *testing.T) {
	// Kubernetes imageID forms all normalize to the bare digest.
	assert.Equal(t, "sha256:abc123", imageDigest("docker-pullable://ghcr.io/org/img@sha256:abc123"))
	assert.Equal(t, "sha256:abc123", imageDigest("ghcr.io/org/img@sha256:abc123"))
	assert.Equal(t, "sha256:abc123", imageDigest("sha256:abc123"))
	// No digest present -> returned unchanged.
	assert.Equal(t, "ghcr.io/org/img:tag", imageDigest("ghcr.io/org/img:tag"))
}

func TestSafeShortID(t *testing.T) {
	assert.Equal(t, "abc", safeShortID("abc"))
	assert.Equal(t, "abcdefghijkl", safeShortID("abcdefghijklmnop"))
	assert.Equal(t, "", safeShortID(""))
}

func TestSanitizeDNS1123(t *testing.T) {
	assert.Equal(t, "run1", sanitizeDNS1123("run1"))
	assert.Equal(t, "my-model", sanitizeDNS1123("My_Model"))
	assert.Equal(t, "a-b", sanitizeDNS1123("a.b"))
	assert.Equal(t, "model", sanitizeDNS1123("___"), "falls back when nothing usable remains")
}

func TestK8sPodName(t *testing.T) {
	name := k8sPodName("/path/to/Run1.mod", "deadbeef12")

	assert.True(t, strings.HasPrefix(name, "janus-hermes-run1-"), "got %q", name)
	assert.True(t, strings.HasSuffix(name, "deadbeef12"), "got %q", name)
	assert.LessOrEqual(t, len(name), 63)

	for _, r := range name {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		assert.True(t, valid, "invalid DNS-1123 char %q in %q", r, name)
	}
}

func TestShortModelHashStable(t *testing.T) {
	h1 := shortModelHash("/a/b/model.mod")
	h2 := shortModelHash("/a/b/model.mod")
	h3 := shortModelHash("/a/b/other.mod")

	assert.Equal(t, h1, h2, "stable for the same path")
	assert.NotEqual(t, h1, h3, "differs across models")
	assert.Len(t, h1, 10)
}

func TestUsesKubernetes(t *testing.T) {
	docker := &HermesExecutor{config: &config.Config{Input: config.Input{Orchestrator: config.OrchestratorDocker}}}
	k8s := &HermesExecutor{config: &config.Config{Input: config.Input{Orchestrator: config.OrchestratorKubernetes}}}

	assert.False(t, docker.usesKubernetes())
	assert.True(t, k8s.usesKubernetes())
	assert.False(t, (&HermesExecutor{}).usesKubernetes(), "nil config is not kubernetes")
}

func TestNewHermesExecutorSkipsDockerOnKubernetes(t *testing.T) {
	modelCfg := &config.HermesExecutionConfig{
		Image:     "ghcr.io/org/img:1",
		Resources: config.ResourceConfig{CPUCores: 1, Memory: "1Gi"},
	}

	// Kubernetes orchestrator must not construct a Docker client (and so must not
	// log "Using configured Docker socket …" on a pure-K8s run).
	k8sCfg := &config.Config{Input: config.Input{
		Orchestrator: config.OrchestratorKubernetes,
		Hermes:       config.HermesConfig{Container: config.HermesContainerConfig{DockerSocket: "npipe:////./pipe/docker_engine"}},
	}}
	k8s := NewHermesExecutor(k8sCfg, modelCfg)
	assert.Nil(t, k8s.dockerClient, "Kubernetes orchestrator should not construct a Docker client")

	// The Docker orchestrator still builds one.
	dockerCfg := &config.Config{Input: config.Input{Orchestrator: config.OrchestratorDocker}}
	docker := NewHermesExecutor(dockerCfg, modelCfg)
	assert.NotNil(t, docker.dockerClient, "Docker orchestrator should construct a Docker client")
}

func TestPodReadyTimeout(t *testing.T) {
	deflt := &HermesExecutor{config: &config.Config{}}
	assert.Equal(t, defaultPodReadyTimeout, deflt.podReadyTimeout())

	configured := &HermesExecutor{config: &config.Config{Input: config.Input{
		Hermes: config.HermesConfig{Container: config.HermesContainerConfig{StartupTimeout: "90s"}},
	}}}
	assert.Equal(t, 90*time.Second, configured.podReadyTimeout())
}
