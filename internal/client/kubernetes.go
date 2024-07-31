package client

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeClient struct {
	*kubernetes.Clientset
}

func NewKubernetesClient(kubeConfigFile string) (*KubeClient, error) {
	config, err := buildConfig(kubeConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &KubeClient{client}, nil
}

func buildConfig(kubeConfigFile string) (*rest.Config, error) {
	if kubeConfigFile == "" {
		// inside kubernetes cluster
		return rest.InClusterConfig()
	}
	// outside kubernetes cluster
	return clientcmd.BuildConfigFromFlags("", kubeConfigFile)
}
