package client

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var kubeTestCases = []struct {
	name         string
	kubeClient   *KubeClient
	resourceYaml []byte
	namespace    string
	imageTag     string
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
                image: nginx:test
        `),
		namespace: "default",
		imageTag:  "test",
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
		imageTag:  "test",
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
		imageTag:  "test",
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
		imageTag:  "test",
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
		imageTag:  "test",
	},
}

func TestDeployDeleteKubeResource(t *testing.T) {
	for _, tc := range kubeTestCases {
		t.Run("DeployResource", func(t *testing.T) {
			ctx := context.Background()
			// Deploy the resource
			labels, replicas, err := tc.kubeClient.Deploy(ctx, tc.resourceYaml, tc.namespace, tc.imageTag)
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

		t.Run("UpdateResource", func(t *testing.T) {
			ctx := context.Background()
			// Deploy the resource again for updating
			labels, replicas, err := tc.kubeClient.Deploy(ctx, tc.resourceYaml, tc.namespace, tc.imageTag)
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

var (
	defaultNamespace        = "default"
	defaultDeploymentLabels = map[string]string{"app": "test"}
)

var waitForPodsRunningTestCases = []struct {
	name             string
	kubeClient       *KubeClient
	namespace        string
	deploymentLabels map[string]string
	pods             []*corev1.Pod
	expectedReplicas int32
}{
	{
		name:             "Test the Waiting for pods to be ready and running",
		kubeClient:       &KubeClient{KubernetesInterface: fake.NewSimpleClientset()},
		namespace:        defaultNamespace,
		deploymentLabels: defaultDeploymentLabels,
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-1",
					Namespace: defaultNamespace,
					Labels:    defaultDeploymentLabels,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-2",
					Namespace: defaultNamespace,
					Labels:    defaultDeploymentLabels,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-3",
					Namespace: defaultNamespace,
					Labels:    defaultDeploymentLabels,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
		expectedReplicas: 3,
	},
}

func TestWaitForPodsRunning(t *testing.T) {
	for _, tc := range waitForPodsRunningTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Create the pods to the fake clientset
			for _, pod := range tc.pods {
				_, err := tc.kubeClient.CoreV1().Pods(tc.namespace).Create(ctx, pod, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create pod: %v", err)
				}
			}

			// Test WaitForPodsRunning
			ctx, cancel := context.WithTimeout(context.TODO(), 120*time.Second)
			defer cancel()

			err := tc.kubeClient.WaitForPodsRunning(ctx, tc.namespace, tc.deploymentLabels, tc.expectedReplicas)
			if err != nil {
				t.Fatalf("WaitForPodsRunning() error = %v", err)
			}
		})
	}
}

var kubeFailureTestCases = []struct {
	name         string
	kubeClient   *KubeClient
	resourceYaml []byte
	namespace    string
	imageTag     string
}{
	{
		name:       "UnsupportedResource",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: v1
        kind: UnsupportedResource
        `),
		namespace: "default",
		imageTag:  "test",
	},
	{
		name:       "PersistentVolumeClaim type",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: v1
        kind: PersistentVolumeClaim
        `),
		namespace: "default",
		imageTag:  "test",
	},
	{
		name:         "nil resource yaml and empty namespace",
		kubeClient:   &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: nil,
		namespace:    "",
		imageTag:     "test",
	},
	{
		name:       "no labels and replicas",
		kubeClient: &KubeClient{KubernetesInterface: fake.NewSimpleClientset()}, // a fake kubernetes clientset
		resourceYaml: []byte(`
        apiVersion: apps/v1
        kind: Deployment
        `),
		namespace: "default",
		imageTag:  "test",
	},
}

func TestKubeFailure(t *testing.T) {
	for _, tc := range kubeFailureTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Deploy the resource
			labels, replicas, err := tc.kubeClient.Deploy(ctx, tc.resourceYaml, tc.namespace, tc.imageTag)
			if err == nil {
				if tc.name == "no labels and replicas" && len(labels) != 0 && replicas != 0 {
					t.Errorf("Deploy() error, expected 0 labels and 0 replicas, got %v labels and %v replicas", len(labels), replicas)
				} else if tc.name != "no labels and replicas" {
					t.Errorf("Deploy() expected error, got none")
				}
			}
		})

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Delete the resource
			err := tc.kubeClient.Delete(ctx, tc.resourceYaml, tc.namespace)
			if err == nil {
				if tc.name != "no labels and replicas" {
					t.Errorf("Delete() expected error, got none")
				}
			}
		})
	}
}

// Test cases for testing timeout during waiting for pods to be running
var waitForPodsTimoutTestCases = []struct {
	name             string
	kubeClient       *KubeClient
	namespace        string
	deploymentLabels map[string]string
	pods             []*corev1.Pod
	expectedReplicas int32
}{
	{
		name:             "Test the Waiting for pods got timeout",
		kubeClient:       &KubeClient{KubernetesInterface: fake.NewSimpleClientset()},
		namespace:        defaultNamespace,
		deploymentLabels: nil,
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: defaultNamespace,
					Labels:    nil,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
		},
		expectedReplicas: 1,
	},
}

func TestTimeoutDuringDeployment(t *testing.T) {
	for _, tc := range waitForPodsTimoutTestCases {
		t.Run("Timeout", func(t *testing.T) {
			ctx := context.Background()
			// Create the pods to the fake clientset
			for _, pod := range tc.pods {
				_, err := tc.kubeClient.CoreV1().Pods(tc.namespace).Create(ctx, pod, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create pod: %v", err)
				}
			}

			// Test WaitForPodsRunning
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			err := tc.kubeClient.WaitForPodsRunning(ctx, tc.namespace, tc.deploymentLabels, tc.expectedReplicas)
			if err == nil {
				t.Fatalf("Expected timeout error during deployment, got nil")
			}
		})
	}
}
