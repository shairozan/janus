package kube

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HermesContainerName is the name of the Hermes container within the pod.
const HermesContainerName = "hermes"

// PodSpec is the orchestrator-agnostic description of the Hermes pod to create.
// Image and resources come from the per-model .janus.config.json; the namespace
// and labels come from Janus.
type PodSpec struct {
	Name            string
	Namespace       string
	Image           string
	Port            int               // container gRPC port (e.g. 50051)
	CPUCores        int               // requested/limited CPU cores
	Memory          string            // memory request/limit, e.g. "8Gi"
	Labels          map[string]string // identification labels (for selection/cleanup)
	ImagePullSecret string            // optional docker-registry secret for private images

	// HermesConfigYAML, when set, is the Hermes server config (command mapping)
	// the orchestrator stores in a ConfigMap and mounts into the pod. The
	// orchestrator sets ConfigMapName after creating it; BuildPod mounts that
	// ConfigMap at hermesConfigMountDir and points HERMES_CONFIG at it.
	HermesConfigYAML string
	ConfigMapName    string
}

// Hermes config mount details — must match where the Hermes server reads its
// config (HERMES_CONFIG) and how the Docker path mounts it.
const (
	hermesConfigVolume   = "hermes-config"
	hermesConfigMountDir = "/etc/hermes"
	hermesConfigFileName = "config.yaml"
	hermesConfigEnvVar   = "HERMES_CONFIG"
)

// BuildPod renders a corev1.Pod for the Hermes workload. The container runs the
// Hermes "serve" entrypoint and exposes the gRPC port; CPU/memory are set as
// both requests and limits so the scheduler reserves what the run needs. The pod
// never restarts (a finished run should not be relaunched by the kubelet).
func BuildPod(s PodSpec) (*corev1.Pod, error) {
	if s.Port <= 0 || s.Port > 65535 {
		return nil, fmt.Errorf("port out of range (1-65535): %d", s.Port)
	}

	resources, err := resourceRequirements(s.CPUCores, s.Memory)
	if err != nil {
		return nil, err
	}

	var pullSecrets []corev1.LocalObjectReference
	if s.ImagePullSecret != "" {
		pullSecrets = []corev1.LocalObjectReference{{Name: s.ImagePullSecret}}
	}

	container := corev1.Container{
		Name:  HermesContainerName,
		Image: s.Image,
		Args:  []string{"serve"},
		Ports: []corev1.ContainerPort{
			{Name: "grpc", ContainerPort: int32(s.Port)}, //nolint:gosec // bounded to 1-65535 above
		},
		Resources: resources,
	}

	var volumes []corev1.Volume

	// Mount the Hermes config (command mapping) from a ConfigMap, mirroring the
	// Docker bind-mount, so Hermes resolves "nonmem" to the configured binary.
	if s.ConfigMapName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: hermesConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: s.ConfigMapName},
				},
			},
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      hermesConfigVolume,
			MountPath: hermesConfigMountDir,
			ReadOnly:  true,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  hermesConfigEnvVar,
			Value: hermesConfigMountDir + "/" + hermesConfigFileName,
		})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    s.Labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:    corev1.RestartPolicyNever,
			ImagePullSecrets: pullSecrets,
			Volumes:          volumes,
			Containers:       []corev1.Container{container},
		},
	}, nil
}

// resourceRequirements builds the container resource requests/limits from a CPU
// core count and a memory quantity string (e.g. "8Gi"). Both are set as request
// and limit so the pod gets a guaranteed reservation.
func resourceRequirements(cpuCores int, memory string) (corev1.ResourceRequirements, error) {
	if cpuCores <= 0 {
		return corev1.ResourceRequirements{}, fmt.Errorf("cpu cores must be positive, got %d", cpuCores)
	}

	cpu, err := resource.ParseQuantity(strconv.Itoa(cpuCores))
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("parsing cpu quantity %q: %w", strconv.Itoa(cpuCores), err)
	}

	mem, err := resource.ParseQuantity(memory)
	if err != nil {
		return corev1.ResourceRequirements{}, fmt.Errorf("parsing memory quantity %q: %w", memory, err)
	}

	list := corev1.ResourceList{
		corev1.ResourceCPU:    cpu,
		corev1.ResourceMemory: mem,
	}

	return corev1.ResourceRequirements{Requests: list, Limits: list}, nil
}
