package kube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// defaultPollInterval is how often Provision polls a pod's status while waiting
// for it to become Ready.
const defaultPollInterval = time.Second

// Orchestrator provisions and tears down a Hermes pod in a namespace and exposes
// it to the local machine via a port-forward, so the existing Hermes gRPC client
// can dial it on localhost unchanged.
type Orchestrator struct {
	clientset    kubernetes.Interface
	restConfig   *rest.Config
	namespace    string
	pollInterval time.Duration
}

// NewOrchestrator builds an Orchestrator for the given namespace. restConfig is
// required for port-forwarding (it carries the API server address and auth).
func NewOrchestrator(clientset kubernetes.Interface, restConfig *rest.Config, namespace string) *Orchestrator {
	return &Orchestrator{
		clientset:    clientset,
		restConfig:   restConfig,
		namespace:    namespace,
		pollInterval: defaultPollInterval,
	}
}

// Workload is a provisioned, reachable Hermes pod.
type Workload struct {
	PodName   string
	LocalAddr string // "127.0.0.1:<localPort>" — dial this with the Hermes gRPC client
	ImageID   string // resolved image reference/digest from the pod status (provenance)

	teardown func()
}

// Teardown stops the port-forward and deletes the pod. Safe to call once.
func (w *Workload) Teardown() {
	if w.teardown != nil {
		w.teardown()
	}
}

// Provision creates the Hermes pod, waits for it to become Ready, and
// port-forwards its gRPC port to a local port. On any failure after the pod is
// created it best-effort deletes the pod before returning.
func (o *Orchestrator) Provision(ctx context.Context, spec PodSpec, readyTimeout time.Duration) (*Workload, error) {
	spec.Namespace = o.namespace

	// Stage the Hermes config (command mapping) in a ConfigMap the pod mounts.
	var configMapName string
	if spec.HermesConfigYAML != "" {
		configMapName = spec.Name + "-config"
		if err := o.createConfigMap(ctx, configMapName, spec.HermesConfigYAML, spec.Labels); err != nil {
			return nil, fmt.Errorf("creating hermes config map: %w", err)
		}

		spec.ConfigMapName = configMapName
	}

	pod, err := BuildPod(spec)
	if err != nil {
		o.deleteConfigMapBestEffort(ctx, configMapName)

		return nil, err
	}

	created, err := o.clientset.CoreV1().Pods(o.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		o.deleteConfigMapBestEffort(ctx, configMapName)

		return nil, fmt.Errorf("creating hermes pod: %w", err)
	}

	podName := created.Name

	ready, err := o.waitForPodReady(ctx, podName, readyTimeout)
	if err != nil {
		o.deletePodBestEffort(ctx, podName)
		o.deleteConfigMapBestEffort(ctx, configMapName)

		return nil, err
	}

	localPort, stopForward, err := o.portForward(ctx, podName, spec.Port)
	if err != nil {
		o.deletePodBestEffort(ctx, podName)
		o.deleteConfigMapBestEffort(ctx, configMapName)

		return nil, fmt.Errorf("port-forwarding hermes pod %s: %w", podName, err)
	}

	return &Workload{
		PodName:   podName,
		LocalAddr: fmt.Sprintf("127.0.0.1:%d", localPort),
		ImageID:   containerImageID(ready),
		teardown: func() {
			stopForward()
			o.deletePodBestEffort(ctx, podName)
			o.deleteConfigMapBestEffort(ctx, configMapName)
		},
	}, nil
}

// DeleteStale removes any pods and config maps in the namespace matching
// labelSelector. Used to clear orphaned Hermes resources from a previous,
// interrupted run.
func (o *Orchestrator) DeleteStale(ctx context.Context, labelSelector string) error {
	pods, err := o.clientset.CoreV1().Pods(o.namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("listing stale hermes pods: %w", err)
	}

	for i := range pods.Items {
		name := pods.Items[i].Name
		// NotFound is success: a pod listed here may already be terminating from a
		// per-stage teardown the safety-net sweep is racing.
		if err := o.clientset.CoreV1().Pods(o.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting stale pod %s: %w", name, err)
		}
	}

	configMaps, err := o.clientset.CoreV1().ConfigMaps(o.namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("listing stale hermes config maps: %w", err)
	}

	for i := range configMaps.Items {
		name := configMaps.Items[i].Name
		if err := o.clientset.CoreV1().ConfigMaps(o.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting stale config map %s: %w", name, err)
		}
	}

	return nil
}

// createConfigMap stores the Hermes config YAML in a ConfigMap the pod mounts.
func (o *Orchestrator) createConfigMap(ctx context.Context, name, yaml string, labels map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: o.namespace, Labels: labels},
		Data:       map[string]string{hermesConfigFileName: yaml},
	}

	_, err := o.clientset.CoreV1().ConfigMaps(o.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating config map %s: %w", name, err)
	}

	return nil
}

// deleteConfigMapBestEffort deletes a config map, ignoring errors and an empty
// name. Detaches from the parent's cancellation so cleanup still runs.
func (o *Orchestrator) deleteConfigMapBestEffort(ctx context.Context, name string) {
	if name == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	_ = o.clientset.CoreV1().ConfigMaps(o.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// waitForPodReady polls the pod until it reports Ready, returning the latest pod
// object. It fails fast if the pod enters a terminal Failed phase.
func (o *Orchestrator) waitForPodReady(ctx context.Context, podName string, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(o.pollInterval)
	defer ticker.Stop()

	for {
		pod, err := o.clientset.CoreV1().Pods(o.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting pod %s: %w", podName, err)
		}

		if podReady(pod) {
			return pod, nil
		}

		if pod.Status.Phase == corev1.PodFailed {
			return nil, fmt.Errorf("hermes pod %s failed to start (%s)", podName, podStatusSummary(pod))
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for pod %s to become ready: %w (%s)", podName, ctx.Err(), podStatusSummary(pod))
		case <-ticker.C:
		}
	}
}

// portForward forwards the pod's podPort to a free local port, returning the
// local port and a stop function. It blocks until the forward is ready.
func (o *Orchestrator) portForward(ctx context.Context, podName string, podPort int) (int, func(), error) {
	roundTripper, upgrader, err := spdy.RoundTripperFor(o.restConfig)
	if err != nil {
		return 0, nil, fmt.Errorf("building spdy round tripper: %w", err)
	}

	reqURL := o.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(o.namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, reqURL)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	// "0:<podPort>" asks kubectl-style port-forwarding to pick a free local port.
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", podPort)}, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return 0, nil, fmt.Errorf("creating port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- fw.ForwardPorts() }()

	select {
	case <-readyCh:
	case err := <-errCh:
		return 0, nil, fmt.Errorf("starting port forward: %w", err)
	case <-ctx.Done():
		close(stopCh)

		return 0, nil, ctx.Err()
	}

	ports, err := fw.GetPorts()
	if err != nil || len(ports) == 0 {
		close(stopCh)

		return 0, nil, fmt.Errorf("resolving forwarded port: %w", err)
	}

	return int(ports[0].Local), func() { close(stopCh) }, nil
}

// deletePodBestEffort deletes a pod, ignoring errors. It detaches from the
// parent's cancellation (via WithoutCancel) but keeps its values, so cleanup
// still runs when the run's context is already done, with its own timeout.
func (o *Orchestrator) deletePodBestEffort(ctx context.Context, podName string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	_ = o.clientset.CoreV1().Pods(o.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// PodLogs returns up to tailLines of the Hermes container's logs, for surfacing
// in a failure message. tailLines <= 0 returns the full log.
func (o *Orchestrator) PodLogs(ctx context.Context, podName string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{Container: HermesContainerName}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	stream, err := o.clientset.CoreV1().Pods(o.namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("opening pod log stream: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading pod logs: %w", err)
	}

	return string(data), nil
}

// podStatusSummary renders a short, human-readable summary of a pod's phase and
// each container's waiting/terminated state, so failures like ImagePullBackOff
// surface a reason instead of just a timeout.
func podStatusSummary(pod *corev1.Pod) string {
	parts := []string{fmt.Sprintf("phase=%s", pod.Status.Phase)}

	for _, cs := range pod.Status.ContainerStatuses {
		switch {
		case cs.State.Waiting != nil:
			parts = append(parts, fmt.Sprintf("%s: waiting (%s: %s)",
				cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message))
		case cs.State.Terminated != nil:
			parts = append(parts, fmt.Sprintf("%s: terminated (%s, exit %d)",
				cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode))
		}
	}

	return strings.Join(parts, "; ")
}

// podReady reports whether a pod is Running with a true Ready condition.
func podReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// containerImageID returns the resolved image reference (with digest, once
// pulled) of the Hermes container, for run-log provenance. Empty if unavailable.
func containerImageID(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == HermesContainerName {
			return cs.ImageID
		}
	}

	return ""
}
