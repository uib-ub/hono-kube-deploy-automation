package client

import (
	"context"
	"fmt"
	"reflect"
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

	log "github.com/sirupsen/logrus"
)

// Define type aliases for Kubernetes resources
type DeploymentType = *appsv1.Deployment
type NamespaceType = *corev1.Namespace
type ConfigMapType = *corev1.ConfigMap
type ServiceType = *corev1.Service
type IngressType = *networkingv1.Ingress

type KubeClient struct {
	*kubernetes.Clientset
}

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

func buildConfig(kubeConfig string) (*rest.Config, error) {
	if kubeConfig == "" {
		// inside kubernetes cluster
		return rest.InClusterConfig()
	}
	// outside kubernetes cluster
	return clientcmd.BuildConfigFromFlags("", kubeConfig)
}

func (k *KubeClient) Deploy(
	ctx context.Context,
	resource []byte,
	ns string,
) (map[string]string, int32, error) {
	// Create a sub-context with a specific timeout to prevent
	// hanging indefinitely, which can lead to deadlocks or resource leaks
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	obj, err := k.decodeResource(resource)
	if err != nil {
		return nil, 0, err
	}
	log.Infof("Deploy resource type: %v", reflect.TypeOf(obj))

	_, err = k.getResource(ctx, ns, obj)
	if err != nil && !errors.IsNotFound(err) {
		return nil, 0, fmt.Errorf("failed to get Kubernetes resource: %w", err)
	}

	if errors.IsNotFound(err) {
		// create deployment of the resource
		return k.handleDeployResource(ctx, ns, obj, true)
	}
	// update deployment of the resource
	return k.handleDeployResource(ctx, ns, obj, false)
}

func (k *KubeClient) Delete(ctx context.Context, resource []byte, ns string) error {
	// Create a sub-context with a specific timeout to prevent
	// hanging indefinitely, which can lead to deadlocks or resource leaks
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	obj, err := k.decodeResource(resource)
	if err != nil {
		return err
	}
	log.Infof("Delete resource type: %v", reflect.TypeOf(obj))
	_, err = k.getResource(ctx, ns, obj)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get Kubernetes resource: %w", err)
	}
	if errors.IsNotFound(err) {
		log.Infof("Kubernetes resource not found, skip deletion")
		return nil
	}

	return k.handleDeleteResource(ctx, ns, obj)
}

func (k *KubeClient) decodeResource(resource []byte) (metav1.Object, error) {
	// Decode the resource
	obj, gvk, err := scheme.Codecs.UniversalDeserializer().Decode(resource, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resource: %w", err)
	}
	log.Debugf("Decoded resource type: %v, kind: %v", reflect.TypeOf(obj), gvk.Kind)
	// Cast object to metav1.Object
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("decoded resource object is not a Kubernetes API object")
	}
	log.Infof("Decoded Kubernetes API object type: %v", reflect.TypeOf(objMeta))

	return objMeta, nil
}

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

func (k *KubeClient) handleDeployResource(
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
		)
	case NamespaceType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().Namespaces().Create,
			k.CoreV1().Namespaces().Update,
		)
	case ConfigMapType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().ConfigMaps(ns).Create,
			k.CoreV1().ConfigMaps(ns).Update,
		)
	case ServiceType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.CoreV1().Services(ns).Create,
			k.CoreV1().Services(ns).Update,
		)
	case IngressType:
		return handleDeployResourceOperation(
			create, obj, ctx,
			k.NetworkingV1().Ingresses(ns).Create,
			k.NetworkingV1().Ingresses(ns).Update,
		)
	default:
		return nil, 0, fmt.Errorf("unsupported Kubernetes resource kind: %v", reflect.TypeOf(obj))
	}
}

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

// Type alias for the Kubernetes resource types.
type KubernetesResource interface {
	DeploymentType | NamespaceType | ConfigMapType | ServiceType | IngressType
}

// Type aliases for function signatures of create and update operations.
type CreateFunc[T any] func(context.Context, T, metav1.CreateOptions) (T, error)
type UpdateFunc[T any] func(context.Context, T, metav1.UpdateOptions) (T, error)

func handleDeployResourceOperation[
	// T DeploymentType | NamespaceType | ConfigMapType | ServiceType | IngressType,
	T KubernetesResource,
](
	create bool,
	obj T,
	ctx context.Context,
	createFunc CreateFunc[T],
	updateFunc UpdateFunc[T],
) (map[string]string, int32, error) {

	var err error
	if create {
		log.Infof("Create Kubernetes resource type: %v ...", reflect.TypeOf(obj))
		_, err = createFunc(ctx, obj, metav1.CreateOptions{})
	} else {
		log.Infof("Update Kubernetes resource type: %v ...", reflect.TypeOf(obj))
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

func getLabels(obj any) map[string]string {
	switch obj := obj.(type) {
	case metav1.Object:
		return obj.GetLabels()
	}
	return nil
}

func getReplicas(obj any) int32 {
	switch obj := obj.(type) {
	case DeploymentType:
		if obj.Spec.Replicas != nil {
			return *obj.Spec.Replicas
		}
	}
	return 0
}
