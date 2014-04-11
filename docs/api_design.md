API Design
----------

The [geard](https://github.com/openshift/geard) agent API is structured around fast and slow idempotent operations - all API responses should finish their primary objective in <10ms with success or failure, and either return immediately with failure, or on success additional data may be streamed to the client in structured (JSON events) or unstructured (logs from journald) form.  In general, all operations should be reentrant - invoking the same operation multiple times with different request ids should yield exactly the same result.  Some operations cannot be repeated because they depend on the state of external resources at a point in time (build of the "master" branch of a git repository) and subsequent operations may not have the same outcome.  These operations should be gated by request identifier where possible, and it is the client's responsibility to ensure that condition holds.  The initial API implementation is in HTTP REST, but we would like to support additional transports and serializations like AMQP, STOMP, ZeroMQ, or Thrift.

The API takes into account the concept of "joining" - if two requests are made with the same request id, where possible the second request should attach to the first job's result and streams in order to provide an identical return value and logs.  This allows clients to design around retries or at-least-once delivery mechanisms safely.  The second job may check the invariants of the first as long as data races can be avoided.

All non-content-streaming jobs (which should already be idempotent and repeatable) should be structured in two phases - execute and report.  The execute phase attempts to set the correct state on the host (systemd unit created, symlinks on disk, data input correct) and to return a 2xx response on success or an error body and 4xx or 5xx response on error as fast as possible.  API operations should *not* wait for asynchronous events like the stable start status of a process, the external ports being bound, or image specific data to be written to disk.  Instead, those are modelled with separate API calls.  The report phase is optional for all jobs, and is where additional data may be streamed to the consumer over HTTP or a message bus.

In general, the philosophy of create/fail fast operations is based around the recognition that distributed systems may fail at any time, but those failures are rare.  If a failure does occur, the recovery path is for a client to retry the operation as originally submitted, or to delete the affected resources, for for a resynchronization to occur.  A service may take several minutes to start only to fail - since failure cannot be predicted, clients should be given tools to recognize and correct failures rather than waiting for an indeterminate period.

At the current time there are no resynchronization operations implemented, but the additional metadata required (vector clocks or consistent versions) should be supportable via the existing interfaces.  An orchestrator would prepare a list of the expected resource state and a reasonably synchronized clock identifier, and the agent would be able to compare that to the persisted resources on disk older than a window. The "repair" functionality on the agent would perform a similar function - ensuring that the set of persisted resources (units, links, port mappings, keys) are internally consistent, and that outside of a minimum window (minutes) any unreferenced content is removed.  This is still an area of active prototyping.


### Concrete example:

Starting a Docker image on a system for the first time may involve several slow steps:

* Downloading the initial image
* Starting the process

Those steps may fail in unpredictable ways - for instance, the service may start but fail due to a configuration error and never begin listening.  A client cannot know for certain the cause of the failure, and so a wait is nondeterministic.  A download may stall for minutes or hours due to network unpredictability, or the local disk may run out of storage during the download and fail (due to other users of the system).

The API forces the client to provide the following info up front:

* A unique locator for the image (which may include the destination from which the image can be fetched)
* The identifier the process will be referenced by in future transactions (so the client can immediately begin dispatching subsequent requests)
* Any initial mapping of network ports or access control configuration for ssh

The API records the effect of this call as a unit file on disk for systemd that can, with no extra input from a client, result in a started process.  The API then returns success and streams the logs to the client.  A client *may* disconnect at this point, without interrupting the operation.  A client may then begin wiring together this process with other processes in the system immediately with the explicit understanding that the endpoints being wired may not yet be available.

In general, systems wired together this way already need to deal with uncertainty of network connectivity and potential startup races.  The API design formalizes that behavior - it is expected that the components "heal" by waiting for their dependencies to become available.  Where possible, the host system will attempt to offer blocking behavior on a per unit basis that allows the logic of the system to be distributed.  In some cases, like TCP and HTTP proxy load balancing, those systems already have mechanisms to tolerate components that may not be started.


