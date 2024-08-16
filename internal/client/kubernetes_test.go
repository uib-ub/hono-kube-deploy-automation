package client

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

var deployTestCases = []struct {
	name         string
	kubeClient   *KubeClient
	resourceYaml []byte
	namespace    string
}{
	{
		name:       "Deploy Deployment",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: test-deployment
          labels:
            app: test
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: test
          template:
            metadata:
              labels:
                app: test
            spec:
              containers:
              - name: test-container
                image: nginx
        `),
		namespace: "default",
	},
	{
		name:       "Deploy Namespace",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: v1
        kind: Namespace
        metadata:
          name: test-namespace
        `),
		namespace: "",
	},
	{
		name:       "Deploy ConfigMap",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: test-configmap
        data:
          key: value
        `),
		namespace: "default",
	},
	{
		name:       "Deploy Service",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: v1
        kind: Service
        metadata:
          name: test-service
        spec:
          selector:
            app: test
          ports:
          - protocol: TCP
            port: 80
            targetPort: 9376
        `),
		namespace: "default",
	},
	{
		name:       "Deploy Service",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          name: test-ingress
        spec:
          rules:
          - host: test.com
            http:
              paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: test-service
                    port:
                      number: 80
        `),
		namespace: "default",
	},
}

func TestDeploy(t *testing.T) {
	for _, tc := range deployTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Deploy the resource
			labels, replicas, err := tc.kubeClient.Deploy(ctx, tc.resourceYaml, tc.namespace)
			if err != nil {
				t.Fatalf("Deploy() error = %v", err)
			}

			// Validate the labels
			if tc.name == "Deploy Deployment" && len(labels) == 0 {
				t.Errorf("Expected labels, got none")
			}

			if tc.name == "Deploy Deployment" && replicas != 3 {
				t.Errorf("Returned replicas = %v, want %v", replicas, 3)
			}
		})
	}
}
