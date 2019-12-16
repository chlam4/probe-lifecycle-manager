# probe-lifecycle-manager
This repo contains a library to abstract out management of probe lifcycles depending on target create/update/delete events.  A probe will be instantiated when the first such target is added, and will be deleted when the last target of the probe type has been deleted.

Optionally, each target info can also be persisted for the local probes to access.
