package manager

import (
	"fmt"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/probe_controller"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/probe_controller/t8c"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar"
	"github.com/turbonomic/probe-lifecycle-manager/pkg/target_registrar/k8s_secret"
	"k8s.io/client-go/rest"
)

// ProbeLifecycleManager manages probe life cycles
type ProbeLifecycleManager struct {
	targetRegistrar target_registrar.Registrar
	probeController probe_controller.ProbeController
}

// DefaultProbeManagerForConfig constructs a default probe lifecycle manager using k8s secrets to keep target info and
// leveraging the t8c operator to control the probes
func DefaultProbeManagerForConfig(kubeConfig *rest.Config, namespace string) (*ProbeLifecycleManager, error) {
	// Instantiate a target registrar, a probe controller and then a probe lifecycle manager
	target_registrar, err := k8s_secret.NewK8sSecretsTargetRegistrarForConfig(kubeConfig, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create a target registrar: %v\n", err)
	}
	probeController, err := t8c.NewT8cProbeControllerForConfig(kubeConfig, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to construct a probe controller: %v\n", err)
	}
	return &ProbeLifecycleManager{targetRegistrar: target_registrar, probeController: probeController}, nil
}

// AddOrUpdateTarget adds or updates the given target and starts the probe if not already started
func (m *ProbeLifecycleManager) AddOrUpdateTarget(target target_registrar.Target) error {
	isFirstTarget, err := m.targetRegistrar.RegisterTarget(target)
	if err != nil {
		return fmt.Errorf("failed to register target %v\n%v", target, err)
	}
	if !isFirstTarget {
		return nil
	}
	return m.probeController.StartProbe(target.GetProbeType())
}

// DeleteTarget deletes the given target and stops the probe if it has no more targets
func (m *ProbeLifecycleManager) DeleteTarget(target target_registrar.Target) error {
	isLastTarget, err := m.targetRegistrar.UnregisterTarget(target)
	if err != nil {
		return err
	}
	if !isLastTarget {
		return nil
	}
	return m.probeController.StopProbe(target.GetProbeType())
}
