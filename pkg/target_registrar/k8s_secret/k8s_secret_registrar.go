package k8s_secret

import (
	"encoding/json"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	clientretry "k8s.io/client-go/util/retry"
)

// K8sSecretsRegistrar implements the Registrar interface using Kubernetes secrets to store target info
type K8sSecretsRegistrar struct {
	client    clientv1.CoreV1Interface
	namespace string
}

// NewK8sSecretsTargetRegistrarForConfig constructs a K8sSecretsRegistrar given the input kubeconfig and the namespace
func NewK8sSecretsTargetRegistrarForConfig(config *rest.Config, namespace string) (*K8sSecretsRegistrar, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewK8sSecretsTargetRegistrarFromClient(kubeClient.CoreV1(), namespace)
}

// NewK8sSecretsTargetRegistrarFromClient constructs a K8sSecretsRegistrar given the kube client and the namespace
func NewK8sSecretsTargetRegistrarFromClient(client clientv1.CoreV1Interface, namespace string) (*K8sSecretsRegistrar, error) {
	return &K8sSecretsRegistrar{
		client:    client,
		namespace: namespace,
	}, nil
}

// TargetToSecret converts the input Target to a k8s secret, using yaml.Marshal()
func TargetToSecret(target target_registrar.Target) (*apiv1.Secret, error) {
	bytes, err := target.Bytes()
	if err != nil {
		return nil, err
	}
	return &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: target.GetProbeType(),
		},
		StringData: map[string]string {
			target.GetId(): string(bytes),
		},
	}, nil
}

// RegisterTarget registers the target by storing its info as a Kubernetes secret.  It returns true if the registration
// is successful and the probe type has now one or more target, or false otherwise.
func (r *K8sSecretsRegistrar) RegisterTarget(target target_registrar.Target) (bool, error) {
	existingSecret, err := r.findSecret(target)
	if err != nil {
		return false, err
	}

	if existingSecret == nil {
		// No existingSecret matches with the probe type of this target; create a new existingSecret
		secret, err := TargetToSecret(target)
		if err != nil {
			return false, err
		}
		existingSecret, err = r.client.Secrets(r.namespace).Create(secret)
		if err == nil {
			return true, nil
		}
		if !errors.IsAlreadyExists(err) {
			return false, err
		}
		// secret already exists: fall through with the existing secret to the following update procedure
	}

	// Secret of this probe type found; update the existingSecret in a separate copy
	newData, err := target.Bytes()
	if err != nil {
		return false, err
	}
	err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		updatedSecret := existingSecret.DeepCopy()
		if updatedSecret.StringData == nil {
			updatedSecret.StringData = map[string]string{}
		}
		updatedSecret.StringData[target.GetId()] = string(newData)
		existingSecret, err = r.patchSecret(existingSecret, updatedSecret)
		return err
	})
	return true, err
}

// UnregisterTarget unregisters the target, by removing the corresponding Kubernetes secret.  It returns true if the
// unregistering has been successful and this probe type has now no more targets, or false otherwise.
func (r *K8sSecretsRegistrar) UnregisterTarget(target target_registrar.Target) (bool, error) {
	existingSecret, err := r.findSecret(target)
	if err != nil {
		return false, err
	}
	if existingSecret == nil {
		// No existingSecret of the probe type found, which essentially means this probe type has no targets
		return true, nil
	}

	// Secret of this probe type found; update the existingSecret in a separate copy
	var updatedSecret *apiv1.Secret
	err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		updatedSecret = existingSecret.DeepCopy()
		delete(updatedSecret.StringData, target.GetId())
		delete(updatedSecret.Data, target.GetId()) // in a real k8s cluster, StringData is converted into Data in []byte form
		existingSecret, err = r.patchSecret(existingSecret, updatedSecret)
		return err
	})
	if err != nil {
		return false, err
	}
	return len(updatedSecret.StringData) == 0 && len(updatedSecret.Data) == 0, nil
}

// findSecret returns the secret associated with the input target if found, or nil if not found
func (r *K8sSecretsRegistrar) findSecret(target target_registrar.Target) (*apiv1.Secret, error) {
	// Fetch the secret associated with this target to decide if this is the first target of this probe type
	selectByNameAsProbeType := fields.OneTermEqualSelector("metadata.name", target.GetProbeType())
	matchedSecrets, err := r.client.Secrets(r.namespace).List(metav1.ListOptions{FieldSelector: selectByNameAsProbeType.String()})
	if err != nil || len(matchedSecrets.Items) == 0 {
		return nil, err
	}
	return &matchedSecrets.Items[0], nil
}

// patchSecret patches a secret to the given new version.  The old version is also passed in to calculate the diff.
func (r *K8sSecretsRegistrar) patchSecret(oldSecret *apiv1.Secret, newSecret *apiv1.Secret) (*apiv1.Secret, error) {
	oldBytes, err := json.Marshal(oldSecret)
	if err != nil {
		return nil, err
	}
	newBytes, err := json.Marshal(newSecret)
	if err != nil {
		return nil, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldBytes, newBytes, apiv1.Secret{})
	if err != nil {
		return nil, err
	}
	return r.client.Secrets(r.namespace).Patch(newSecret.GetName(), types.StrategicMergePatchType, patchBytes)
}
