package client

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var kubeTestCases = []struct {
	name         string
	kubeClient   *KubeClient
	resourceYaml []byte
	namespace    string
}{
	{
		name:       "Deployment",
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
		name:       "Namespace",
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
		name:       "ConfigMap",
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
		name:       "Service",
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
		name:       "Ingress",
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

func TestDeployDeleteKubeResource(t *testing.T) {
	for _, tc := range kubeTestCases {
		t.Run("DeployResource", func(t *testing.T) {
			ctx := context.Background()

			// Deploy the resource
			labels, replicas, err := tc.kubeClient.Deploy(ctx, tc.resourceYaml, tc.namespace)
			if err != nil {
				t.Fatalf("Deploy() error = %v", err)
			}

			// Validate the labels
			if tc.name == "Deployment" && len(labels) == 0 {
				t.Errorf("Deploy() error, expected labels, got none")
			}

			if tc.name == "Deployment" && replicas != 3 {
				t.Errorf("Deploy() error, returned replicas = %v, want %v", replicas, 3)
			}
		})

		t.Run("DeleteResource", func(t *testing.T) {
			ctx := context.Background()
			// Delete the resource
			err := tc.kubeClient.Delete(ctx, tc.resourceYaml, tc.namespace)
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			// Validate deletion based on resource kind
			switch tc.name {
			case "Deployment":
				_, err = tc.kubeClient.AppsV1().Deployments(tc.namespace).Get(ctx, "test-deployment", metav1.GetOptions{})
			case "Namespace":
				_, err = tc.kubeClient.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{})
			case "ConfigMap":
				_, err = tc.kubeClient.CoreV1().ConfigMaps(tc.namespace).Get(ctx, "test-configmap", metav1.GetOptions{})
			case "Service":
				_, err = tc.kubeClient.CoreV1().Services(tc.namespace).Get(ctx, "test-service", metav1.GetOptions{})
			case "Ingress":
				_, err = tc.kubeClient.NetworkingV1().Ingresses(tc.namespace).Get(ctx, "test-ingress", metav1.GetOptions{})
			}

			// Expect an error indicating the resource is not found
			if err == nil {
				t.Errorf("Expected resource to be deleted, but it still exists")
			} else if !errors.IsNotFound(err) {
				t.Errorf("Unexpected error when checking deletion: %v", err)
			}
		})
	}
}
