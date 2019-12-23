package t8c_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/probe_controller/t8c"
	v1beta1fake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var (
	testNamespace = "turbonomic"
)

var _ = Describe("Test turbo probe controller", func() {
	DescribeTable("test starting a probe",
		func(probeType string, existingSpec map[string]interface{}) {
			dynamicClient := dynamicfake.NewSimpleDynamicClient(t8c.Scheme)
			v1beta1Client := v1beta1fake.FakeApiextensionsV1beta1{Fake: &dynamicClient.Fake}
			cr, gvr, err := t8c.GetOrCreateCR(&v1beta1Client, dynamicClient, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = unstructured.SetNestedMap(cr.Object, existingSpec, "spec")
			Expect(err).NotTo(HaveOccurred())

			probeController := t8c.NewT8cProbeControllerFromClient(&v1beta1Client, dynamicClient, testNamespace)
			err = probeController.StartProbe(probeType)
			Expect(err).NotTo(HaveOccurred())

			result, err := dynamicClient.Resource(*gvr).Namespace(testNamespace).Get(t8c.XlCrDefaultName, metav1.GetOptions{})
			value, found, err := unstructured.NestedBool(result.Object, "spec", probeType, "enabled")
			Expect(found).To(Equal(true))
			Expect(value).To(Equal(true))
		},
		Entry("start a probe when none exists", "vcenter", map[string]interface{}{}),
		Entry("start a probe when it has already started", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": true},
		}),
		Entry("start a probe that is disabled/stopped", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": false},
		}),
		Entry("start a probe when other probes are present", "vcenter", map[string]interface{}{
			"pure" : map[string]interface{}{"enabled": true},
			"appdynamics" : map[string]interface{}{"enabled": false},
		}),
		Entry("start a probe when it is stopped and other probes are present", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": false},
			"pure" : map[string]interface{}{"enabled": true},
			"appdynamics" : map[string]interface{}{"enabled": false},
		}),
	)
	DescribeTable("test stopping a probe",
		func(probeType string, existingSpec map[string]interface{}) {
			dynamicClient := dynamicfake.NewSimpleDynamicClient(t8c.Scheme)
			v1beta1Client := v1beta1fake.FakeApiextensionsV1beta1{Fake: &dynamicClient.Fake}
			cr, gvr, err := t8c.GetOrCreateCR(&v1beta1Client, dynamicClient, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = unstructured.SetNestedMap(cr.Object, existingSpec, "spec")
			Expect(err).NotTo(HaveOccurred())

			probeController := t8c.NewT8cProbeControllerFromClient(&v1beta1Client, dynamicClient, testNamespace)
			err = probeController.StopProbe(probeType)
			Expect(err).NotTo(HaveOccurred())

			result, err := dynamicClient.Resource(*gvr).Namespace(testNamespace).Get(t8c.XlCrDefaultName, metav1.GetOptions{})
			value, found, err := unstructured.NestedBool(result.Object, "spec", probeType, "enabled")
			Expect(found).To(Equal(true))
			Expect(value).To(Equal(false))
		},
		Entry("stop a probe when none exists", "vcenter", map[string]interface{}{}),
		Entry("stop a probe when it was started", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": true},
		}),
		Entry("stop a probe that has already been stopped", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": false},
		}),
		Entry("stop a probe when it is started and other probes are present", "vcenter", map[string]interface{}{
			"vcenter" : map[string]interface{}{"enabled": true},
			"pure" : map[string]interface{}{"enabled": true},
			"appdynamics" : map[string]interface{}{"enabled": false},
		}),
	)
})
