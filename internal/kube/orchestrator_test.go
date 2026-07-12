package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func readyPod(name, ns string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: HermesContainerName, ImageID: "docker-pullable://hermes@sha256:abc123"},
			},
		},
	}
}

func TestWaitForPodReady(t *testing.T) {
	cs := fake.NewClientset(readyPod("p", "janus"))
	o := NewOrchestrator(cs, nil, "janus")
	o.pollInterval = 5 * time.Millisecond

	pod, err := o.waitForPodReady(context.Background(), "p", time.Second)
	require.NoError(t, err)
	assert.Equal(t, "docker-pullable://hermes@sha256:abc123", containerImageID(pod))
}

func TestWaitForPodReadyTimeout(t *testing.T) {
	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "janus"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	cs := fake.NewClientset(pending)
	o := NewOrchestrator(cs, nil, "janus")
	o.pollInterval = 5 * time.Millisecond

	_, err := o.waitForPodReady(context.Background(), "p", 40*time.Millisecond)
	assert.Error(t, err)
}

func TestWaitForPodReadyFailsFast(t *testing.T) {
	failed := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "janus"},
		Status:     corev1.PodStatus{Phase: corev1.PodFailed},
	}
	cs := fake.NewClientset(failed)
	o := NewOrchestrator(cs, nil, "janus")
	o.pollInterval = 5 * time.Millisecond

	_, err := o.waitForPodReady(context.Background(), "p", time.Second)
	assert.Error(t, err)
}

func TestDeleteStale(t *testing.T) {
	mine := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "a", Namespace: "janus", Labels: map[string]string{"app": "janus-hermes"},
	}}
	other := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "b", Namespace: "janus", Labels: map[string]string{"app": "something-else"},
	}}
	cs := fake.NewClientset(mine, other)
	o := NewOrchestrator(cs, nil, "janus")

	require.NoError(t, o.DeleteStale(context.Background(), "app=janus-hermes"))

	remaining, err := cs.CoreV1().Pods("janus").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, remaining.Items, 1)
	assert.Equal(t, "b", remaining.Items[0].Name, "only the unlabeled pod should remain")
}

func TestDeletePodBestEffort(t *testing.T) {
	cs := fake.NewClientset(readyPod("p", "janus"))
	o := NewOrchestrator(cs, nil, "janus")

	o.deletePodBestEffort(context.Background(), "p")

	_, err := cs.CoreV1().Pods("janus").Get(context.Background(), "p", metav1.GetOptions{})
	assert.Error(t, err, "pod should be gone")

	// Deleting a non-existent pod must not panic.
	o.deletePodBestEffort(context.Background(), "does-not-exist")
}

func TestConfigMapLifecycle(t *testing.T) {
	cs := fake.NewClientset()
	o := NewOrchestrator(cs, nil, "janus")

	require.NoError(t, o.createConfigMap(context.Background(), "p-config", "yaml: content", map[string]string{"app": "janus-hermes"}))

	cm, err := cs.CoreV1().ConfigMaps("janus").Get(context.Background(), "p-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "yaml: content", cm.Data[hermesConfigFileName])

	o.deleteConfigMapBestEffort(context.Background(), "p-config")
	_, err = cs.CoreV1().ConfigMaps("janus").Get(context.Background(), "p-config", metav1.GetOptions{})
	assert.Error(t, err, "config map should be gone")

	// Empty name is a no-op and must not panic.
	o.deleteConfigMapBestEffort(context.Background(), "")
}

func TestDeleteStaleClearsConfigMaps(t *testing.T) {
	mine := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "a-config", Namespace: "janus", Labels: map[string]string{"app": "janus-hermes"},
	}}
	other := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "keep", Namespace: "janus", Labels: map[string]string{"app": "something-else"},
	}}
	cs := fake.NewClientset(mine, other)
	o := NewOrchestrator(cs, nil, "janus")

	require.NoError(t, o.DeleteStale(context.Background(), "app=janus-hermes"))

	remaining, err := cs.CoreV1().ConfigMaps("janus").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, remaining.Items, 1)
	assert.Equal(t, "keep", remaining.Items[0].Name)
}

func TestPodStatusSummary(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: HermesContainerName,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "ImagePullBackOff", Message: "Back-off pulling image",
			}},
		}},
	}}

	summary := podStatusSummary(pod)
	assert.Contains(t, summary, "phase=Pending")
	assert.Contains(t, summary, "ImagePullBackOff")
	assert.Contains(t, summary, HermesContainerName)
}

func TestPodReady(t *testing.T) {
	assert.True(t, podReady(readyPod("p", "n")))
	assert.False(t, podReady(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}))
	// Running but no Ready condition yet.
	assert.False(t, podReady(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}))
}
