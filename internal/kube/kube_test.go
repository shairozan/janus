package kube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildPod(t *testing.T) {
	pod, err := BuildPod(PodSpec{
		Name:      "janus-run1",
		Namespace: "janus",
		Image:     "pharmalytica/hermes-nonmem:nm76",
		Port:      50051,
		CPUCores:  4,
		Memory:    "8Gi",
		Labels:    map[string]string{"app": "janus-hermes", "model": "run1"},
	})
	require.NoError(t, err)

	assert.Equal(t, "janus-run1", pod.Name)
	assert.Equal(t, "janus", pod.Namespace)
	assert.Equal(t, "janus-hermes", pod.Labels["app"])
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)

	require.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	assert.Equal(t, HermesContainerName, c.Name)
	assert.Equal(t, "pharmalytica/hermes-nonmem:nm76", c.Image)
	assert.Equal(t, []string{"serve"}, c.Args)

	require.Len(t, c.Ports, 1)
	assert.Equal(t, int32(50051), c.Ports[0].ContainerPort)

	// CPU and memory are set as both requests and limits.
	assert.Equal(t, "4", c.Resources.Limits.Cpu().String())
	assert.Equal(t, "8Gi", c.Resources.Limits.Memory().String())
	assert.Equal(t, "4", c.Resources.Requests.Cpu().String())
	assert.Equal(t, "8Gi", c.Resources.Requests.Memory().String())
}

func TestBuildPodImagePullSecret(t *testing.T) {
	// No secret -> no ImagePullSecrets.
	pod, err := BuildPod(PodSpec{Name: "p", Namespace: "n", Image: "i", Port: 50051, CPUCores: 1, Memory: "1Gi"})
	require.NoError(t, err)
	assert.Empty(t, pod.Spec.ImagePullSecrets)

	// Secret set -> referenced on the pod.
	pod, err = BuildPod(PodSpec{
		Name: "p", Namespace: "n", Image: "ghcr.io/org/img:1", Port: 50051,
		CPUCores: 1, Memory: "1Gi", ImagePullSecret: "ghcr-creds",
	})
	require.NoError(t, err)
	require.Len(t, pod.Spec.ImagePullSecrets, 1)
	assert.Equal(t, "ghcr-creds", pod.Spec.ImagePullSecrets[0].Name)
}

func TestBuildPodConfigMapMount(t *testing.T) {
	// No ConfigMap -> no volume/mount/env.
	pod, err := BuildPod(PodSpec{Name: "p", Namespace: "n", Image: "i", Port: 50051, CPUCores: 1, Memory: "1Gi"})
	require.NoError(t, err)
	assert.Empty(t, pod.Spec.Volumes)
	assert.Empty(t, pod.Spec.Containers[0].VolumeMounts)
	assert.Empty(t, pod.Spec.Containers[0].Env)

	// ConfigMap set -> volume, mount at /etc/hermes, and HERMES_CONFIG env.
	pod, err = BuildPod(PodSpec{
		Name: "p", Namespace: "n", Image: "i", Port: 50051, CPUCores: 1, Memory: "1Gi",
		ConfigMapName: "p-config",
	})
	require.NoError(t, err)

	require.Len(t, pod.Spec.Volumes, 1)
	assert.Equal(t, "p-config", pod.Spec.Volumes[0].ConfigMap.Name)

	mounts := pod.Spec.Containers[0].VolumeMounts
	require.Len(t, mounts, 1)
	assert.Equal(t, "/etc/hermes", mounts[0].MountPath)

	env := pod.Spec.Containers[0].Env
	require.Len(t, env, 1)
	assert.Equal(t, "HERMES_CONFIG", env[0].Name)
	assert.Equal(t, "/etc/hermes/config.yaml", env[0].Value)
}

func TestBuildPodInvalid(t *testing.T) {
	_, err := BuildPod(PodSpec{Name: "x", Namespace: "n", Image: "i", Port: 50051, CPUCores: 0, Memory: "8Gi"})
	assert.Error(t, err, "zero cpu cores should be rejected")

	_, err = BuildPod(PodSpec{Name: "x", Namespace: "n", Image: "i", Port: 50051, CPUCores: 4, Memory: "not-a-quantity"})
	assert.Error(t, err, "unparseable memory should be rejected")
}

const testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster:
    server: https://one.test:6443
- name: c2
  cluster:
    server: https://two.test:6443
contexts:
- name: ctx-one
  context:
    cluster: c1
    user: u
- name: ctx-two
  context:
    cluster: c2
    user: u
users:
- name: u
  user: {}
current-context: ctx-one
`

func writeKubeconfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testKubeconfig), 0o600))

	return path
}

func TestRESTConfigCurrentContext(t *testing.T) {
	cfg, err := RESTConfig(writeKubeconfig(t), "")
	require.NoError(t, err)
	assert.Equal(t, "https://one.test:6443", cfg.Host)
}

func TestRESTConfigContextOverride(t *testing.T) {
	cfg, err := RESTConfig(writeKubeconfig(t), "ctx-two")
	require.NoError(t, err)
	assert.Equal(t, "https://two.test:6443", cfg.Host)
}

func TestRESTConfigMissingKubeconfig(t *testing.T) {
	_, err := RESTConfig(filepath.Join(t.TempDir(), "does-not-exist"), "")
	assert.Error(t, err)
}
