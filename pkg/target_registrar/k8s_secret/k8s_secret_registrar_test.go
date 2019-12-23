package k8s_secret_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar/k8s_secret"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/testing"
)

var (
	testNamespace = "turbonomic"
)

// getFakeClient constructs a fake Kubernetes client and injects all the existing targets into the client.  It also
// constructs the filtered list of secrets to return for the list action called on the fake client - this is because
// the fake client doesn't have the server-side filtering logic implemented and our functions being tested here rely on
// the list-filtering capabilities on the server side
func getFakeClient(existingTargets []target_registrar.Target, probeTypeToFilter string) (corev1.CoreV1Interface, error) {
	// Convert all existing targets into secrets; this assumes existing secrets aren't overlapping in terms of probe
	// type.  Otherwise, the construction logic needs to be enhanced.
	var allExistingSecrets []apiv1.Secret
	var filteredSecrets []apiv1.Secret
	for _, tgt := range existingTargets {
		secret, err := k8s_secret.TargetToSecret(tgt)
		if err != nil {
			return nil, err
		}
		allExistingSecrets = append(allExistingSecrets, *secret)
		if tgt.GetProbeType() == probeTypeToFilter {
			filteredSecrets = append(filteredSecrets, *secret)
		}
	}

	// Create a fake k8s client set, and inject all existing secrets.  Also add a reactor for list action to
	// return only the filtered list for this test purpose.
	fakeClientSet := fake.NewSimpleClientset()
	fakeClientSet.Fake.PrependReactor("list", "secrets",
		func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			switch action.(type) {
			case testing.ListAction:
				var secretList apiv1.SecretList
				secretList.Items = filteredSecrets
				return true, &secretList, nil
			}
			return false, nil, nil
		})
	client := fakeClientSet.CoreV1()
	for _, secret := range allExistingSecrets {
		_, err := client.Secrets(testNamespace).Create(&secret)
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

// filterTargets iterates the given list of targets and filters out those with the same type and id as the targetToSkip
func filterTargets(targetsToFilter []target_registrar.Target, targetToSkip target_registrar.Target) []target_registrar.Target {
	var result []target_registrar.Target
	for _, tgt := range targetsToFilter {
		if tgt.GetProbeType() != targetToSkip.GetProbeType() || tgt.GetId() != targetToSkip.GetId() {
			result = append(result, tgt)
		}
	}
	return result
}

// checkTargets checks each target in the to-check list and ensure it can be retrieved back from the fake client
func checkTargets(client corev1.CoreV1Interface, targetsToCheck []target_registrar.Target) {
	for _, expectedTarget := range targetsToCheck {
		// Read back the corresponding secret
		secret, err := client.Secrets(testNamespace).Get(expectedTarget.GetProbeType(), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Unmarshal the secret to retrieve the target info
		retrievedBytes := []byte (secret.StringData[expectedTarget.GetId()])
		var retrievedTarget target_registrar.UserPassTarget
		err = target_registrar.UserPassTargetFromBytes(retrievedBytes, &retrievedTarget)
		Expect(err).NotTo(HaveOccurred())
		Expect(retrievedTarget).To(Equal(expectedTarget))
	}
}

var _ = Describe("Test k8s secret registrar", func() {
	DescribeTable("test registering a target",
		func(existingTargets []target_registrar.Target, newTarget target_registrar.Target) {
			client, err := getFakeClient(existingTargets, newTarget.GetProbeType())
			Expect(err).NotTo(HaveOccurred())

			targetRegistrar, err := k8s_secret.NewK8sSecretsTargetRegistrarFromClient(client, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Register the new expectedTarget
			probeTypeHasTarget, err := targetRegistrar.RegisterTarget(newTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(probeTypeHasTarget).To(Equal(true))

			// Retrieve secrets from the fake client to confirm all targets supposed to be there are there
			targetsToCheck := filterTargets(existingTargets, newTarget)
			targetsToCheck = append(targetsToCheck, newTarget)
			checkTargets(client, targetsToCheck)
		},
		Entry("create a new secret for the target when no secrets exist yet", []target_registrar.Target{},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("update the corresponding secret for the new target when a target already exists of the same probe type",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid2", Probetype:"vcenter", Username:"user2", Password:"pass2"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("create a new secret for the target when another secret exists for a different probe type",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid2", Probetype:"pure", Username:"user2", Password:"pass2"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("update a target that exists already",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user2", Password:"pass2"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
	)

	DescribeTable("test unregistering a target",
		func(existingTargets []target_registrar.Target, targetToUnregister target_registrar.Target) {
			client, err := getFakeClient(existingTargets, targetToUnregister.GetProbeType())
			Expect(err).NotTo(HaveOccurred())

			targetRegistrar, err := k8s_secret.NewK8sSecretsTargetRegistrarFromClient(client, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Unregister the target
			probeTypeHasNoTarget, err := targetRegistrar.UnregisterTarget(targetToUnregister)
			Expect(err).NotTo(HaveOccurred())
			// Check if we expect the corresponding probe type still has a target
			expectedNoTarget := true
			for _, tgt := range existingTargets {
				if tgt.GetProbeType() == targetToUnregister.GetProbeType() && tgt.GetId() != targetToUnregister.GetId() {
					expectedNoTarget = false
				}
			}
			Expect(probeTypeHasNoTarget).To(Equal(expectedNoTarget))

			// Retrieve secrets from the fake client to confirm all targets supposed to be there are there
			targetsToCheck := filterTargets(existingTargets, targetToUnregister)
			checkTargets(client, targetsToCheck)
		},
		Entry("try to delete a target when no secrets exist", []target_registrar.Target{},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("try to delete a target when a different target exists of the same probe type",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid2", Probetype:"vcenter", Username:"user2", Password:"pass2"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("try to delete a target when only a secret of a different probe type exists",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid2", Probetype:"pure", Username:"user2", Password:"pass2"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
		Entry("delete the only target of the probe type",
			[]target_registrar.Target{target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}},
			target_registrar.UserPassTarget{Id: "Moid1", Probetype:"vcenter", Username:"user1", Password:"pass1"}),
	)
})
