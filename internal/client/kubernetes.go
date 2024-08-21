package client

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typednetworkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"

	log "github.com/sirupsen/logrus"
	"github.com/uib-ub/hono-kube-deploy-automation/internal/util"
)

// Define type aliases for Kubernetes resources
type DeploymentType = *appsv1.Deployment
type NamespaceType = *corev1.Namespace
type ConfigMapType = *corev1.ConfigMap
type ServiceType = *corev1.Service
type IngressType = *networkingv1.Ingress

// KubernetesInterface defines the methods we use from the Kubernetes clientset
type KubernetesInterface interface {
	AppsV1() typedappsv1.AppsV1Interface
	CoreV1() typedcorev1.CoreV1Interface
	NetworkingV1() typednetworkingv1.NetworkingV1Interface
}

// Ensure that kubernetes.Clientset implements KubernetesInterface
var _ KubernetesInterface = &kubernetes.Clientset{}

// KubeClient struct accepts an interface that both kubernetes.Clientset and fake.Clientset implement.
// This approach allows you to use both real and fake clients interchangeably.
type KubeClient struct {
	// *kubernetes.Clientset
	KubernetesInterface
}

// NewKubernetesClient creates a new KubeClient using the provided kubeConfig.
// If kubeConfig is empty, it attempts to create an in-cluster configuration.
func NewKubernetesClient(kubeConfig string) (*KubeClient, error) {
	config, err := buildConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &KubeClient{client}, nil
}

// buildConfig constructs a Kubernetes client configuration based on the provided kubeConfig.
// If kubeConfig is empty, it returns an in-cluster configuration.
func buildConfig(kubeConfig string) (*rest.Config, error) {
	if kubeConfig == "" {
		// Use in-cluster config if no kubeconfig fie is provided
		return rest.InClusterConfig()
	}
	// Use provided kubeconfig file for outside the Kubernetes cluster
	return clientcmd.BuildConfigFromFlags("", kubeConfig)
}

// Deploy deploys or updates a Kubernetes resource in the specified namespace.
func (k *KubeClient) Deploy(
	ctx context.Context,
	resource []byte,
	ns string,
	imageTag string,
) (map[string]string, int32, error) {
	// Create a sub-context with a specific timeout to prevent
	// hanging indefinitely, which can lead to deadlocks or resource leaks
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	// Decode the Kubernetes resource from the provided byte slice.
	obj, err := k.decodeResource(resource)
	if err != nil {
		return nil, 0, err
	}
	log.Infof("Deploy resource type: %v", reflect.TypeOf(obj))

	// Check if the resource already exists.
	_, err = k.getResource(ctx, ns, obj)
	if err != nil && !errors.IsNotFound(err) {
		return nil, 0, fmt.Errorf("failed to get Kubernetes resource: %w", err)
	}

	// If the resource doesn't exist, create it; otherwise, update it.
	if errors.IsNotFound(err) {
		log.Info("Kubernetes resource not found, creating ...")
		return k.handleDeployResource(imageTag, ctx, ns, obj, true) // true for create
	}
	log.Info("Kubernetes resource found, updating ...")
	return k.handleDeployResource(imageTag, ctx, ns, obj, false) // false for update
}

// Delete removes a Kubernetes resource from the specified namespace.
func (k *KubeClient) Delete(ctx context.Context, resource []byte, ns string) error {
	// Create a sub-context with a specific timeout to prevent
	// hanging indefinitely, which can lead to deadlocks or resource leaks
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	// Decode the Kubernetes resource from the provided byte slice.
	obj, err := k.decodeResource(resource)
	if err != nil {
		return err
	}
	log.Infof("Delete resource type: %v", reflect.TypeOf(obj))
	_, err = k.getResource(ctx, ns, obj)

	// Check if the resource exists before attempting to delete it.
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get Kubernetes resource: %w", err)
	}
	if errors.IsNotFound(err) {
		log.Infof("Kubernetes resource not found, skip deletion")
		return nil
	}

	// If the resource exists, delete it.
	return k.handleDeleteResource(ctx, ns, obj)
}

// decodeResource decodes a Kubernetes resource from a byte slice.
func (k *KubeClient) decodeResource(resource []byte) (metav1.Object, error) {
	// Decode the resource into a Kubernetes API object.
	obj, gvk, err := scheme.Codecs.UniversalDeserializer().Decode(resource, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resource: %w", err)
	}
	log.Debugf("Decoded resource type: %v, kind: %v", reflect.TypeOf(obj), gvk.Kind)

	// Cast the decoded object to a metav1.Object, which represents a Kubernetes resource.
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("decoded resource object is not a Kubernetes API object")
	}
	log.Infof("Decoded Kubernetes API object type: %v", reflect.TypeOf(objMeta))

	return objMeta, nil
}

// getResource retrieves a Kubernetes resource by its type and namespace.
func (k *KubeClient) getResource(
	ctx context.Context,
	ns string,
	obj metav1.Object,
) (metav1.Object, error) {
	switch obj := obj.(type) {
	case DeploymentType:
		return k.AppsV1().Deployments(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
	case NamespaceType:
		return k.CoreV1().Namespaces().Get(ctx, obj.GetName(), metav1.GetOptions{})
	case ConfigMapType:
		return k.CoreV1().ConfigMaps(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
	case ServiceType:
		return k.CoreV1().Services(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
	case IngressType:
		return k.NetworkingV1().Ingresses(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported Kubernetes resource kind: %v", reflect.TypeOf(obj))
	}
}

// handleDeployResource handles the creation or update of a Kubernetes resource.
func (k *KubeClient) handleDeployResource(
	imageTag string,
	ctx context.Context,
	ns string,
	obj metav1.Object,
	create bool,
) (map[string]string, int32, error) {
	switch obj := obj.(type) {
	case DeploymentType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.AppsV1().Deployments(ns).Create,
			k.AppsV1().Deployments(ns).Update,
			imageTag,
		)
	case NamespaceType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().Namespaces().Create,
			k.CoreV1().Namespaces().Update,
			imageTag,
		)
	case ConfigMapType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().ConfigMaps(ns).Create,
			k.CoreV1().ConfigMaps(ns).Update,
			imageTag,
		)
	case ServiceType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().Services(ns).Create,
			k.CoreV1().Services(ns).Update,
			imageTag,
		)
	case IngressType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.NetworkingV1().Ingresses(ns).Create,
			k.NetworkingV1().Ingresses(ns).Update,
			imageTag,
		)
	default:
		return nil, 0, fmt.Errorf("unsupported Kubernetes resource kind: %v", reflect.TypeOf(obj))
	}
}

// handleDeleteResource handles the deletion of a Kubernetes resource.
func (k *KubeClient) handleDeleteResource(
	ctx context.Context,
	ns string,
	obj metav1.Object,
) error {
	switch obj := obj.(type) {
	case DeploymentType:
		return k.AppsV1().Deployments(ns).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	case NamespaceType:
		return k.CoreV1().Namespaces().Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	case ConfigMapType:
		return k.CoreV1().ConfigMaps(ns).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	case ServiceType:
		return k.CoreV1().Services(ns).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	case IngressType:
		return k.NetworkingV1().Ingresses(ns).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	default:
		return fmt.Errorf("unsupported Kubernetes resource kind: %v", reflect.TypeOf(obj))
	}
}

// Type alias for the Kubernetes resource types to simplify the function signatures.
type KubernetesResource interface {
	DeploymentType | NamespaceType | ConfigMapType | ServiceType | IngressType
}

// Type aliases for function signatures of create and update operations.
type CreateFunc[T any] func(context.Context, T, metav1.CreateOptions) (T, error)
type UpdateFunc[T any] func(context.Context, T, metav1.UpdateOptions) (T, error)

// handleDeployResourceOperation is a generic function that handles the creation or updating
// of a Kubernetes resource. The type parameter T represents the specific Kubernetes resource type.
func handleDeployResourceOperation[
	T KubernetesResource, // T must be one of the Kubernetes resource types being handled.
](
	create bool,
	obj T,
	ctx context.Context,
	createFunc CreateFunc[T],
	updateFunc UpdateFunc[T],
	imageTag string,
) (map[string]string, int32, error) {

	var err error
	if create {
		log.Infof("Create Kubernetes resource type: %v ...", reflect.TypeOf(obj))
		_, err = createFunc(ctx, obj, metav1.CreateOptions{})
	} else {
		log.Infof("Update Kubernetes resource type: %v ...", reflect.TypeOf(obj))
		triggerRollingRestart(obj, imageTag)
		_, err = updateFunc(ctx, obj, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, 0, fmt.Errorf(
			"failed to handle Kubernetes resource type %v: %w",
			reflect.TypeOf(obj),
			err,
		)
	}
	return getLabels(obj), getReplicas(obj), nil
}

func triggerRollingRestart(obj any, imageTag string) {
	switch obj := obj.(type) {
	case DeploymentType:
		currentImage := obj.Spec.Template.Spec.Containers[0].Image
		if strings.Contains(currentImage, imageTag) {
			log.Infof("Image tag %s already exists in deployment %s", imageTag, obj.GetName())
			// Initialize the Annotations map if it's nil
			if obj.Spec.Template.Annotations == nil {
				obj.Spec.Template.Annotations = make(map[string]string)
			}
			// Trigger a rolling restart by updating an annotation
			obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
			log.Infof("Triggering rolling restart for deployment %s", obj.GetName())
		}
	}
}

// getLabels is a generic function that retrieves the labels from a Kubernetes resource object.
// It accepts any object type that implements the metav1.Object interface.
func getLabels(obj any) map[string]string {
	switch obj := obj.(type) {
	case metav1.Object:
		return obj.GetLabels()
	}
	return nil
}

// getReplicas is a generic function that retrieves the replica count from a Kubernetes Deployment object.
// It accepts any object type but only returns the replica count for DeploymentType.
func getReplicas(obj any) int32 {
	switch obj := obj.(type) {
	case DeploymentType:
		if obj.Spec.Replicas != nil {
			return *obj.Spec.Replicas
		}
	}
	return 0
}

// WaitForPodsRunning waits until all pods associated with a deployment are running.
func (k *KubeClient) WaitForPodsRunning(
	ctx context.Context,
	ns string,
	deploymentLabels map[string]string,
	expectedPods int32,
) error {
	// Create a ticker that triggers every 60 seconds.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop() // Ensure the ticker is stopped when we're done.

	util.NotifyLog("Start checking pods status very 60 seconds...")
	for {
		// When the ticker ticks, perform the pod status check.
		labelSelector, err := labels.ValidatedSelectorFromSet(deploymentLabels)
		if err != nil {
			log.WithError(err).Error("Failed to create label selector")
			return fmt.Errorf("create label selector failure: %w", err)
		}
		podList, err := k.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
		if err != nil {
			log.WithError(err).Error("Failed to list pods")
			return fmt.Errorf("list pods failure: %w", err)
		}
		// Count how many of the listed pods are in the "Running" phase.
		podsRunning := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				podsRunning++
			}
		}

		log.Infof("Waiting for %s pods for namespace %s to be running: %d/%d\n", labelSelector.String(), ns, podsRunning, len(podList.Items))
		util.NotifyLog("Waiting for %s pods for namespace %s to be running: %d/%d\n", labelSelector.String(), ns, podsRunning, len(podList.Items))
		// Check if the number of running pods matches the expected count.
		// If all expected pods are running, return successfully.
		if podsRunning > 0 && podsRunning == len(podList.Items) && podsRunning == int(expectedPods) {
			return nil
		}

		select {
		case <-ctx.Done():
			// If the context is cancelled or times out, return an error.
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-ticker.C:
			// Wait for the next tick of the ticker.
		}
	}
}
