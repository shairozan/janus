// Package kube provides thin, testable helpers for talking to Kubernetes via
// client-go: building a REST config / clientset from a kubeconfig + context, and
// rendering the Hermes pod spec. It deliberately takes primitive parameters (not
// Janus config types) so it stays decoupled from the rest of the app.
package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// RESTConfig builds a *rest.Config from a kubeconfig path and context.
//
// An empty kubeconfigPath uses client-go's standard resolution: the KUBECONFIG
// environment variable, then ~/.kube/config. An empty context uses the
// kubeconfig's current-context.
func RESTConfig(kubeconfigPath, context string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		rules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig (path=%q, context=%q): %w", kubeconfigPath, context, err)
	}

	return cfg, nil
}

// NewClientset builds a Kubernetes clientset from a kubeconfig path and context.
func NewClientset(kubeconfigPath, context string) (*kubernetes.Clientset, error) {
	cfg, err := RESTConfig(kubeconfigPath, context)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes clientset: %w", err)
	}

	return clientset, nil
}
