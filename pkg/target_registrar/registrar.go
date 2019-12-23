package target_registrar

// Registrar declares the interface to register and unregister target info
type Registrar interface {
	// RegisterTarget registers the target, usually keeping the target info somewhere.  It returns true if there is at
	// least one target for the probe, which should always be true unless there is an error.
	RegisterTarget(target Target) (bool, error)
	// UnregisterTarget unregisters the target.  It returns true if there is no more target for this probe, or false
	// otherwise.
	UnregisterTarget(target Target) (bool, error)
}