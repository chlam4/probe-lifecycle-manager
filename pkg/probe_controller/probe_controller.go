package probe_controller

// ProbeController is the interface defining a list of actions to control probes such as start probe and stop probe.
type ProbeController interface {
	// StartProbe starts a probe if not yet started
	StartProbe(probeType string) error
	// StopProbe stops a probe if it is started
	StopProbe(probeType string) error
}

