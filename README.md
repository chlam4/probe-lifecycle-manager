# probe-lifecycle-manager

This is a library to manage probe lifecycle based on whether a corresponding target is added.  A probe will be 
enabled/started when at least one corresponding target exists, and will be disabled/stopped when there are no 
corresponding targets.

This simple implementation of probe lifecycle management relies on two capabilities: the first is a capability to 
start and stop the probes, and the second capability is to keep track of the targets being added and removed.  In this 
implementation we have abstracted these two capabilities.  We call the first capability a 
[probe controller](pkg/probe_controller), who is responsible for starting and stopping the probes.  We provide an 
implementation of this capability for the Turbonomic XL platform, using the means of the t8c operator.

We call the second capability a [target registrar](pkg/target_registrar), who is responsible for keeping the target 
info and answering the question whether any target exists for a probe.  This answer helps the lifecycle manager to 
decide whether to start or stop a probe.  For this capability, we provide an implementation of keeping the target info 
as Kubernetes secrets.

Please see an overall example [here](examples/example.go) how to use this library.