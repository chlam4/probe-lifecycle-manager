package t8c

import (
	"fmt"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	clientretry "k8s.io/client-go/util/retry"
)

// T8cProbeController is a probe controller for the Turbonomic XL platform using the XL custom resource
type T8cProbeController struct {
	v1beta1Client v1beta1.ApiextensionsV1beta1Interface
	dynamicClient dynamic.Interface
	namespace     string
}

// NewT8cProbeControllerForConfig constructs a T8cProbeController given the input kubeconfig
func NewT8cProbeControllerForConfig(config *rest.Config, namespace string) (*T8cProbeController, error) {
	v1betaClient, err := v1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewT8cProbeControllerFromClient(v1betaClient, dynamicClient, namespace), nil
}

// NewT8cProbeControllerForConfig constructs a T8cProbeController given the clients
func NewT8cProbeControllerFromClient(v1beta1Client v1beta1.ApiextensionsV1beta1Interface,
	dynamicClient dynamic.Interface, namespace string) *T8cProbeController {
	return &T8cProbeController{
		v1beta1Client: v1beta1Client,
		dynamicClient: dynamicClient,
		namespace:     namespace,
	}
}

// Start a probe in the Kubernetes cluster by simply setting it to enabled in the deployment CR
func (pc *T8cProbeController) StartProbe(probeType string) error {
	return pc.setEnabledFlag(probeType, true)
}

// Stop a probe in the Kubernetes cluster by simply setting it to disabled in the deployment CR
func (pc *T8cProbeController) StopProbe(probeType string) error {
	return pc.setEnabledFlag(probeType, false)
}

// Set the enabled flag for a probe in the deployment CR
func (pc *T8cProbeController) setEnabledFlag(probeType string, enabled bool) error {
	cr, gvr, err := GetOrCreateCR(pc.v1beta1Client, pc.dynamicClient, pc.namespace)
	if err != nil {
		return fmt.Errorf("failed to enable probe %v in namespace %v\n%v", probeType, pc.namespace, err)
	}
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		if err = unstructured.SetNestedField(cr.Object, enabled, "spec", probeType, "enabled"); err != nil {
			return fmt.Errorf("failed to set probe %v to enabled in CR %v in namespace %v\n%v", probeType, cr, pc.namespace, err)
		}
		cr, err = pc.dynamicClient.Resource(*gvr).Namespace(pc.namespace).Update(cr, metav1.UpdateOptions{})
		return err
	})
}
