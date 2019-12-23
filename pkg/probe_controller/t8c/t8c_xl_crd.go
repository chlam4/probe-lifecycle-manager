package t8c

import (
	"fmt"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	clientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	clientretry "k8s.io/client-go/util/retry"
	"strings"
)

// XL CRD in yaml form; probably should just be generated from
const XlCrdYaml = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: xls.charts.helm.k8s.io
spec:
  group: charts.helm.k8s.io
  names:
    kind: Xl
    listKind: XlList
    plural: xls
    singular: xl
  scope: Namespaced
  subresources:
    status: {}
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
`
const XlCrDefaultName = "xl-release"

var (
	Scheme = runtime.NewScheme()
	defaultCrd, defaultGvk, parseErr = parseXlCrd()
)

// parseXlCrd parses the CRD and returns the GroupVersionKind and the unstructured object
func parseXlCrd() (*v1beta1.CustomResourceDefinition, *schema.GroupVersionKind, error) {
	install.Install(Scheme)
	decode := serializer.NewCodecFactory(Scheme).UniversalDeserializer().Decode
	obj, gvk, err := decode([]byte(XlCrdYaml), nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode the t8c XL CRD: %v\n%v", err, XlCrdYaml)
	}
	return obj.(*v1beta1.CustomResourceDefinition), gvk, nil
}

// CreateCRD creates the Turbonomic XL custom resource definition if not already created
func getOrCreateCRD(client clientv1beta1.ApiextensionsV1beta1Interface) (*v1beta1.CustomResourceDefinition, error) {
	if parseErr != nil {
		return nil, fmt.Errorf("failed to get/create the t8c XL CRD\n%v", parseErr)
	}
	gvr := schema.GroupVersionResource{Group: defaultGvk.Group, Version: defaultGvk.Version, Resource: strings.ToLower(defaultGvk.Kind+"s")}
	selectByName := fields.OneTermEqualSelector("metadata.name", defaultCrd.Name)
	crdList, err := client.CustomResourceDefinitions().List(metav1.ListOptions{FieldSelector: selectByName.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to get/create the t8c XL CRD without being able to list (gvr=%v): %v", gvr, err)
	}
	if len(crdList.Items) > 0 {
		// CRD already created
		return &crdList.Items[0], nil
	}

	// Create one and then retrieve it back to confirm
	if _, err := client.CustomResourceDefinitions().Create(defaultCrd); err != nil {
		return nil, fmt.Errorf("failed to create the t8c XL CRD (gvr=%v)\n%v", gvr, err)
	}
	var crd *v1beta1.CustomResourceDefinition	// this will be filled below
	return crd, clientretry.OnError(clientretry.DefaultRetry, func(err error) bool {
		return errors.IsNotFound(err)
	}, func() error {
		crd, err = client.CustomResourceDefinitions().Get(defaultCrd.Name, metav1.GetOptions{})
		return err
	})
}

// getGvrFromCrd constructs the GroupVersionResource info of the custom resource from the CRD.  This involves picking
// a served version out of a possible list.  If no served version if found, return an error.
func getGvrFromCrd(crd *v1beta1.CustomResourceDefinition) (*schema.GroupVersionResource, error) {
	// Spec.Version and Spec.Versions can both be populated, but the former is to be deprecated.
	versionChosen := crd.Spec.Version
	for _, version := range crd.Spec.Versions {
		// The list of versions is sorted with the latest first;
		// so simply choose the first one from the list that is marked served.
		if version.Served {
			versionChosen = version.Name
			break
		}
	}
	if versionChosen == "" {
		return nil, fmt.Errorf("failed to construct the GroupVersionResource without a valid served version from the CRD: %v", crd)
	}
	return &schema.GroupVersionResource{Group: crd.Spec.Group, Version: versionChosen, Resource: crd.Spec.Names.Plural}, nil
}

// GetOrCreateCR retrieves a XL CR from the given namespace if one exists.  If multiple exist, then the first one on
// the list will be returned.  If none exists, then a default will be created.
func GetOrCreateCR(v1beta1Client clientv1beta1.ApiextensionsV1beta1Interface, dynamicClient dynamic.Interface,
	namespace string) (*unstructured.Unstructured, *schema.GroupVersionResource, error) {

	crd, err := getOrCreateCRD(v1beta1Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get/create the t8c XL resource without the CRD: %v", err)
	}
	gvr, err := getGvrFromCrd(crd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get/create the t8c XL resource without being able to construct the GroupVersionResource from CRD (crd=%v)\n%v", crd, err)
	}
	// Look for any existing CR; return it if found
	crList, err := dynamicClient.Resource(*gvr).Namespace(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get/create the t8c XL resource without being able to retrieve the list: %v", err)
	}
	if len(crList.Items) > 0 {
		// at least one found; return the first one
		return &crList.Items[0], gvr, nil
	}

	// None found; create one with the default name
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gvr.Group + "/" + gvr.Version,
			"kind":       crd.Spec.Names.Kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      XlCrDefaultName,
			},
		},
	}
	cr, err = dynamicClient.Resource(*gvr).Namespace(namespace).Create(cr, metav1.CreateOptions{});
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create the t8c XL resource: %v", err)
	}
	return cr, gvr, nil
}