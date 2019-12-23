package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/manager"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	demoNs = "probe-lifecycle-demo"
)

// This demonstrates adding, updating and then deleting targets
// To run: go build example.go --kubeconfig <path-to-your-kubeconfig-file>
func main() {
	// Parse kubeconfig
	var kubeConfigPath string
	pflag.CommandLine.StringVar(&kubeConfigPath, "kubeconfig", kubeConfigPath, "Path to kubeconfig file.")
	pflag.Parse()
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		panic(fmt.Errorf("fatal error: failed to get kubeconfig: %v", err))
	}

	// Create a demo namespace
	if err = createDemoNamespace(kubeConfig); err != nil {
		panic(fmt.Errorf("failed to create a demo namespace: %v", err))
	}

	// Instantiate a default probe manager using k8s secrets to store and track target info and
	probeManager, err := manager.DefaultProbeManagerForConfig(kubeConfig, demoNs)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate a probe manager: %v", err))
	}

	// Add a few targets, 3 vCenter targets, 1 Pure Storage target, 1 AppDynamics target
	vcTarget1 := target_registrar.UserPassTarget{Id: "moid1", Probetype: "vcenter", Username: "testuser1", Password: "testpass1"}
	if err = probeManager.AddOrUpdateTarget(vcTarget1); err != nil {
		panic(fmt.Errorf("failed to add target %v\n%v", vcTarget1, err))
	}
	vcTarget2 := target_registrar.UserPassTarget{Id: "moid2", Probetype: "vcenter", Username: "testuser2", Password: "testpass2"}
	if err = probeManager.AddOrUpdateTarget(vcTarget2); err != nil {
		panic(fmt.Errorf("failed to add target %v\n%v", vcTarget2, err))
	}
	vcTarget3 := target_registrar.UserPassTarget{Id: "moid3", Probetype: "vcenter", Username: "testuser3", Password: "testpass3"}
	if err = probeManager.AddOrUpdateTarget(vcTarget3); err != nil {
		panic(fmt.Errorf("failed to add target %v\n%v", vcTarget3, err))
	}
	pureTarget := target_registrar.UserPassTarget{Id: "moid4", Probetype: "pure", Username: "testuser4", Password: "testpass4"}
	if err = probeManager.AddOrUpdateTarget(pureTarget); err != nil {
		panic(fmt.Errorf("failed to add target %v\n%v", pureTarget, err))
	}
	appdTarget := target_registrar.UserPassTarget{Id: "moid5", Probetype: "appdynamics", Username: "testuser5", Password: "testpass5"}
	if err = probeManager.AddOrUpdateTarget(appdTarget); err != nil {
		panic(fmt.Errorf("failed to add target %v\n%v", appdTarget, err))
	}
	// Update the first vCenter target with a different password
	vcTarget1update := target_registrar.UserPassTarget{Id: "moid1", Probetype: "vcenter", Username: "testuser1", Password: "testpass9"}
	if err = probeManager.AddOrUpdateTarget(vcTarget1update); err != nil {
		panic(fmt.Errorf("failed to update target %v\n%v", vcTarget1update, err))
	}
	// Delete the second vCenter target and the AppD target
	if err = probeManager.DeleteTarget(vcTarget2); err != nil {
		panic(fmt.Errorf("failed to delete target %v\n%v", vcTarget2, err))
	}
	if err = probeManager.DeleteTarget(appdTarget); err != nil {
		panic(fmt.Errorf("failed to delete target %v\n%v", appdTarget, err))
	}

	// Use kubectl to examine the remaining targets and the corresponding probe status -
	// There should be 2 vCenter targets and 1 Pure Storage target left.  Both the vCenter and the Pure Storage probes
	// should be enabled and the AppD probe should be disabled.
	//
	fmt.Println("Issue the following commands to examine the remaining targets and the corresponding probe status")
	fmt.Println("1) Check the list of vCenter targets in secret: there should only be two left (moid1 and moid3)")
	fmt.Println("   kubectl -n probe-lifecycle-demo get secret vcenter -oyaml")
	fmt.Println("1a) Decode vCenter target 1 to confirm the password has changed")
	fmt.Println("    kubectl -n probe-lifecycle-demo get secret vcenter -ojson | jq -j '.data.moid1' | base64 --decode")
	fmt.Println("1b) Decode and check vCenter target 3")
	fmt.Println("    kubectl -n probe-lifecycle-demo get secret vcenter -ojson | jq -j '.data.moid3' | base64 --decode")
	fmt.Println("2) Check the list of probes in the t8c XL resource: vcenter/pure are enabled and appdynamics is disabled ")
	fmt.Println("   kubectl -n probe-lifecycle-demo get xl xl-release -ojson | jq -j '.spec'")
}

// createDemoNamespace creates a demo namespace if not yet created
func createDemoNamespace(kubeConfig *rest.Config) error {
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get kube client from config: %v", err)
	}

	// Create the demo namespace if not already created
	selectByName := fields.OneTermEqualSelector("metadata.name", demoNs)
	matchedNamespaces, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{FieldSelector: selectByName.String()})
	if err != nil {
		return fmt.Errorf("failed to retrieve the list of namespaces: %v", err)
	}
	if len(matchedNamespaces.Items) == 0 {
		// Demo namespace is not found; create it
		nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: demoNs}}
		if _, err = kubeClient.CoreV1().Namespaces().Create(nsSpec); err != nil {
			return fmt.Errorf("failed to retrieve the list of namespaces: %v", err)
		}
	}
	return nil
}