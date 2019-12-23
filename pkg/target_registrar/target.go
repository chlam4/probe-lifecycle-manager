package target_registrar

// Target is the interface outlining the contract between this probe lifecycle manager and the client who uses it
type Target interface {
	//GetProbeType returns the probe type associated with this target for discovery
	GetProbeType() string
	// GetId returns a unique id for this target
	GetId() string
	// Bytes returns a byte array encoding this target's info
	Bytes() ([]byte, error)
}