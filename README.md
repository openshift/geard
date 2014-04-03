geard [![Build Status](https://travis-ci.org/openshift/geard.png?branch=master)](https://travis-ci.org/openshift/geard)
=====

geard is an opinionated tool for installing Docker images as containers onto a systemd-enabled Linux operating system (systemd 207 or newer).  It may be run as a command:

    $ sudo gear install pmorie/sti-html-app my-sample-service

to install the public image <code>pmorie/sti-html-app</code> to systemd on the local box with the service name "ctr-my-sample-service".  The command can also start as a daemon and serve API requests over HTTP (port 43273 is the default):

    $ sudo gear daemon
    2014/02/21 02:59:42 ports: searching block 41, 4000-4099
    2014/02/21 02:59:42 Starting HTTP on :43273 ...

You can also use the gear command against a remote daemon:

    $ gear stop localhost/my-sample-service
    $ gear install pmorie/sti-html-app localhost/my-sample-service.1 localhost/my-sample-service.2
    $ gear start localhost/my-sample-service.1 localhost/my-sample-service.2

The gear daemon and local commands must run as root to interface with the Docker daemon over its Unix socket and systemd over DBus.

geard exposes *primitives* for dealing with containers across hosts and is intended to work closely with a Docker installation - as the plugin system in Docker evolves, many of these primitives may move into plugins of Docker itself.

### What's a gear?

A gear is an isolated Linux container and is an evolution of the SELinux jails used in OpenShift.  For those familiar with Docker, it's a started container with some bound ports, some shared environment, some linking, some resource isolation and allocation, and some opionated defaults about configuration that ease use.  Here's some of those defaults:

1. **Gears are isolated from each other and the host, except where they're explicitly connected**

   By default, a container doesn't have access to the host system processes or files, except where an administrator explicitly chooses, just like Docker.

2. **Gears are portable across hosts**

   A gear, like a Docker image, should be usable on many different hosts.  This means that the underlying Docker abstractions (links, port mappings, environment files) should be used to ensure the gear does not become dependent on the host system.  The system should make it easy to share environment and context between gears and move them among host systems.

3. **Systemd is in charge of starting and stopping gears and journald is in charge of log aggregation**

   A Linux container (Docker or not) is just a process.  No other process manager is as powerful or flexible as systemd, so it's only natural to depend on systemd to run processes and Docker to isolate them.  All of the flexibility of systemd should be available to customize gears, with reasonable defaults to make it easy to get started.

4. **By default, every gear is quota bound and security constrained**

   An isolated gear needs to minimize its impact on other gears in predictable ways.  Leveraging a host user id (uid) per gear allows the operating system to impose limits to file writes, and using SELinux MCS category labels ensures that processes and files in different gears are strongly separated.  An administrator might choose to share some of these limits, but by default enforcing them is good.

   A consequence of per gear uids is that each container can be placed in its own user namespace - the users within the container might be defined by the image creator, but the system sees a consistent user.

5. **The default network configuration of a container is simple**

   By default a container will have 0..N ports exposed and the system will automatically allocate those ports.  An admin may choose to override or change those mappings at runtime, or apply rules to the system that are applied each time a new gear is added.  Much of the linking between containers is done over the network or the upcoming Beam constructs in Docker.


### Actions on a container

Here are the initial set of supported container actions - these should map cleanly to Docker, systemd, or a very simple combination of the two.  Geard unifies the services, but does not reinterpret them.

*   Create a new system unit file that runs a single docker image (install and start a container)

        $ gear install pmorie/sti-html-app localhost/my-sample-service --start
        $ curl -X PUT "http://localhost:43273/container/my-sample-service" -H "Content-Type: application/json" -d '{"Image": "pmorie/sti-html-app", "Started":true}'

*   Stop, start, and restart a container

        $ gear stop localhost/my-sample-service
        $ curl -X PUT "http://localhost:43273/container/my-sample-service/stopped"
        $ gear start localhost/my-sample-service
        $ curl -X PUT "http://localhost:43273/container/my-sample-service/started"
        $ gear restart localhost/my-sample-service
        $ curl -X POST "http://localhost:43273/container/my-sample-service/restart"

*   Deploy a set of containers on one or more systems, with links between them:

        # create a simple two container web app
        $ gear deploy deployment/fixtures/simple_deploy.json localhost

    The links between containers are iptables based rules - try curling 127.0.0.1:8081 to see the second web container.

        # create a mongo db replica set (some assembly required)
        $ gear deploy deployment/fixtures/mongo_deploy.json localhost
        $ sudo switchns db-1 /bin/bash
        > mongo 192.168.1.1
        MongoDB shell version: 2.4.9
        > rs.initiate({_id: "replica0", version: 1, members:[{_id: 0, host:"192.168.1.1:27017"}]})
        > rs.add("192.168.1.2")
        > rs.add("192.168.1.3")
        > rs.status()
        # wait....
        > rs.status()

    The argument to initiate() sets the correct hostname for the first member, otherwise the other members cannot connect.

*   View the systemd status of a container

        $ gear status localhost/my-sample-service
        $ curl "http://localhost:43273/container/my-sample-service/status"

*   Tail the logs for a container (will end after 30 seconds)

        $ curl "http://localhost:43273/container/my-sample-service/log"

*   List all installed containers (for one or more servers)

        $ gear list-units localhost
        $ curl "http://localhost:43273/containers"

*   Create a new empty Git repository

        $ curl -X PUT "http://localhost:43273/repository/my-sample-repo"

*   Link containers with local loopback ports (for e.g. 127.0.0.2:8081 -> 9.8.23.14:8080). If local ip isn't specified, it defaults to 127.0.0.1

        $ gear link -n=127.0.0.2:8081:9.8.23.14:8080 localhost/my-sample-service

*   Set a public key as enabling SSH or Git SSH access to a container or repository (respectively)

        $ gear keys --key-file=[FILE] my-sample-service
        $ curl -X POST "http://localhost:43273/keys" -H "Content-Type: application/json" -d '{"Keys": [{"Type":"authorized_keys","Value":"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ=="}], "Containers": [{"Id": "my-sample-service"}]}'

*   Enable SSH access to join a container for a set of authorized keys (requires 'gear install --isolate')

        TODO: add fixture public and private key for example

*   Build a new image from a source URL and base image

        $ curl -X POST "http://localhost:43273/build-image" -H "Content-Type: application/json" -d '{"BaseImage":"pmorie/fedora-mock","Source":"git://github.com/pmorie/simple-html","Tag":"mybuild-1"}'

*   Fetch a Git archive zip for a repository

        $ curl "http://localhost:43273/repository/my-sample-repo/archive/master"

*   Set and retrieve environment files for sharing between containers (patch and pull operations)

        $ gear set-env localhost/my-sample-service A=B B=C
        $ gear env localhost/my-sample-service
        $ curl "http://localhost:43273/environment/my-sample-service"
        $ gear set-env localhost/my-sample-service --reset

    You can set environment during installation

        $ gear install ccoleman/envtest localhost/env-test1 --env-file=deployment/fixtures/simple.env

    Loading environment into a running container is dependent on the "docker run --env-file" option in Docker master from 0.9.x after April 1st.  You must start the daemon with "gear daemon --has-env-file" in order to use the option - this option will be made the default after 0.9.1 lands and the minimal requirements will be updated.

*   More to come....

geard allows an administrator to easily ensure a given Docker container will *always* run on the system by creating a systemd unit describing a docker run command.  It will execute the Docker container processes as children of the systemd unit, allowing auto restart of the container, customization of additional namespace options, the capture stdout and stderr to journald, and audit/seccomp integration to those child processes.

Note: foreground execution is currently not in Docker master - see https://github.com/alexlarsson/docker/tree/forking-run for some prototype work demonstrating the concept.

Each created systemd unit can be assigned a unique Unix user for quota and security purposes.  An SELinux MCS category label will automatically be assigned to the container to separate it from the other containers on the system, and containers can be set into  systemd slices with resource constraints.

A container may also be optionally enabled for public key SSH access for a set of known keys under the user identifier associated with the container.  On SSH to the host, they'll join the running namespace for that container.


How can geard be used in orchestration?
---------------------------------------

geard is intended to be useful in different scales of container management:

* as a simple command line tool that can quickly generate new unit files and complement the systemctl command line
* as a component in a large distributed infrastructure under the control of a central orchestrator
* as an extensible component for other forms of orchestration

As this is a wide range of scales to satisfy, the core operations are designed to be usable over most common transports - including HTTP, message queues, and gossip protocols.  The default transport is HTTP, and a few operations like log streaming, transferring large binary files, or waiting for operations to complete are best modeled by direct HTTP calls to a given server.  The remaining calls expect to receive a limited set of input and then effect changes to the state of the system - operations like install, delete, stop, and start.  In many cases these are simple passthrough calls to the systemd DBus API and persist additional data to disk (described below).  However, other orchestration styles like pull-from-config-server could implement a transport that would watch the config server for changes and then invoke those fundamental primitives.

One responsibility of the transport layer is authentication and authorization of incoming requests - it is expected that transports would be configured to handle that responsibility and that geard would be minimally aware of the identity of the sender.  Under HTTPS that might be provided by a distributed public key infrastructure (PKI) where the orchestrator has a private key and can sign those requests with hosts that are configured to trust the orchestrator public key (via client certs).  Over a message bus such as ActiveMQ or Qpid TLS and queue permissions would perform much the same function.

From the gear CLI, you can perform operations directly as root (use the embedded gear API library code) or connect to one or more geard instances over HTTP (or another transport).  This works well for managing a few servers or interacting with a subset of hosts in a larger system.

![cli_topologies](./docs/simple_cli_topology.png "CLI interactions with the server")

At larger scales, an orchestrator component is required to implement features like automatic rebalancing of hosts, failure detection, and autoscaling.  The different types of orchestrators and some of their limitations are shown in the diagrams below:

![orchestration_topologies](./docs/orchestration_topologies.png "Orchestration styles and limitations")

As noted, the different topologies have different security and isolation characteristics - generally you trade ease of setup and ease of distributing changes for increasing host isolation.  At the extreme, a large multi-tenant provider may want to minimize the risks of host compromise by preventing nodes from being able to talk to each other, except when the orchestrator delegates.  The encrypted/ package demonstrates one way of doing host delegation - a signed, encrypted token which only the orchestrator can generate, but hosts can validate.  The orchestrator can then give node 1 a token which allows it to call an API on node 2.

A second part of securing large clusters is ensuring the data flowing back to the orchestrator can be properly attributed - if a host is compromised it should not be able to write data onto a shared message bus that masquerades as other hosts, or to execute commands on those other hosts.  This usually means a request-reply pattern (such as implemented by MCollective over STOMP) where requests are read off one queue and written to another, and the caller is responsible for checking that responses match valid requests.

On the other end of the spectrum, in small clusters ease of setup is the gating factor and there tend to be less extreme multi-tenant security concerns.  A [gossip network](http://www.serfdom.io) or distributed config server like [etcd](https://github.com/coreos/etcd) can integrate with geard to serve as both data store and transport layer.


Try it out
----------

The geard code depends on:

* systemd 207 (Fedora 20 or newer)
* Docker 0.7 or newer (0.9.x from Apr 1 to use --env-file, various other experimental features not in tree)

If you don't have those, you can use the following to run in a development vm:

* Vagrant
* VirtualBox

If you have Go installed locally (have a valid GOPATH env variable set), run:

    go get github.com/openshift/geard
    cd $GOPATH/src/github.com/openshift/geard
    vagrant up

If you don't have Go installed locally, run the following steps:

    git clone git@github.com:openshift/geard && cd geard
    vagrant up

The `geard` project is set up such that `vagrant up` will download and install `geard`'s dependencies as well as installing and enabling the systemd unit to run `geard` in a docker container under systemd.

Once `vagrant up` is running, you can ssh into the vm:

    vagrant ssh

The `contrib/build` script allows you to build and run the project in two different ways:

1.  Build binaries locally and run the daemon interactively with `gear daemon`
1.  Build a docker image and run the containerized daemon as a systemd unit

### Building and running locally

To build and run locally, run the following commands in an ssh session to your development vm:

    contrib/build -s
    sudo ./gear daemon

The gear daemon's logs will go to the console in this case.

### Building and running in a container

To build a Docker image and run the containerized daemon as a systemd unit:

    contrib/build -d

This will build the Docker image and start the geard.service systemd unit (as well as restart the unit if it is already running).

See [contrib/example.sh](contrib/example.sh) and [contrib/stress.sh](contrib/stress.sh) for more examples of API calls.


Concepts
--------

Outline of how some of the core operations work:

* Linking - use iptable rules and environment variables to simplify container interconnect
* SSH - generate authorized_keys for a user on demand
* Isolated container - start an arbitrary image and force it to run as a given user on the host by chown the image prior to execution
* Idling - use iptable rules to wake containers on SYN packets
* Git - host Git repositories inside a running Docker container
* Logs - stream journald log entries to clients
* Builds - use transient systemd units to execute a build inside a container
* Jobs - run one-off jobs as systemd transient units and extract their logs and output after completion

Not yet implemented:

* Integrated health check - mark containers as available once a pluggable/configurable health check passes
* Joining - reconnect to an already running operation
* Direct server to server image pulls - allow hosts to act as a distributed registry
* Job callbacks - invoke a remote endpoint after an operation completes
* Local routing - automatically distribute config for inbound and outbound proxying via HAProxy
* Repair - cleanup and perform consistency checks on stored data (most operations assume some cleanup)
* Capacity reporting - report capacity via API calls, allow precondition PUTs based on remaining capacity ("If-Match: capacity>=5"), allow capacity to be defined via config


API Design
----------

The API is structured around fast and slow idempotent operations - all API responses should finish their primary objective in <10ms with success or failure, and either return immediately with failure, or on success additional data may be streamed to the client in structured (JSON events) or unstructured (logs from journald) form.  In general, all operations should be reentrant - invoking the same operation multiple times with different request ids should yield exactly the same result.  Some operations cannot be repeated because they depend on the state of external resources at a point in time (build of the "master" branch of a git repository) and subsequent operations may not have the same outcome.  These operations should be gated by request identifier where possible, and it is the client's responsibility to ensure that condition holds.

The API takes into account the concept of "joining" - if two requests are made with the same request id, where possible the second request should attach to the first job's result and streams in order to provide an identical return value and logs.  This allows clients to design around retries or at-least-once delivery mechanisms safely.  The second job may check the invariants of the first as long as data races can be avoided.

All non-content-streaming jobs (which should already be idempotent and repeatable) will eventually be structured in two phases - execute and report.  The execute phase attempts to assert that the state on the host is accurate (systemd unit created, symlinks on disk, data input correct) and to return a 2xx response on success or an error body and 4xx or 5xx response on error as fast as possible.  API operations should *not* wait for asynchronous events like the stable start status of a process, the external ports being bound, or image specific data to be written to disk.  Instead, those are modelled with separate API calls.  The report phase is optional for all jobs, and is where additional data may be streamed to the consumer over HTTP or a message bus.

In general, the philosophy of create/fail fast operations is based around the recognition that distributed systems may fail at any time, but those failures are rare.  If a failure does occur, the recovery path is for a client to retry the operation as originally submitted, or to delete the affected resources, for for a resynchronization to occur.  A service may take several minutes to start only to fail - since failure cannot be predicted, clients should be given tools to recognize and correct failures.

At the current time there are no resynchronization operations implemented, but the additional metadata (vector clocks or consistent versions) for that should be supportable via the existing interfaces.  An orchestrator would prepare a list of the expected resource state and a reasonably synchronized clock identifier, and the agent would be able to compare that to the persisted resources on disk older than a window. The "repair" functionality on the agent would perform a similar function - ensuring that the set of persisted resources (units, links, port mappings, keys) are internally consistent, and that outside of a minimum window (minutes) any unreferenced content is removed.


### Concrete example:

Starting a Docker image on a system for the first time may involve several slow steps:

* Downloading the initial image
* Starting the process

Those steps may fail in unpredictable ways - for instance, the service may start but fail due to a configuration error and never begin listening.  A client cannot know for certain the cause of the failure (unless they've solved the Halting Problem), and so a wait is nondeterministic.  A download may stall for minutes or hours due to network unpredictability, or the local disk may run out of storage during the download and fail (due to other users of the system).

The API forces the client to provide the following info up front:

* A unique locator for the image (which may include the destination from which the image can be fetched)
* The identifier the process will be referenced by in future transactions (so the client can immediately begin dispatching subsequent requests)
* Any initial mapping of network ports or access control configuration for ssh

The API records the effect of this call as a unit file on disk for systemd that can, with no extra input from a client, result in a started process.  The API then returns success and streams the logs to the client.  A client *may* disconnect at this point, without interrupting the operation.  A client may then begin wiring together this process with other processes in the system immediately with the explicit understanding that the endpoints being wired may not yet be available.

In general, systems wired together this way already need to deal with uncertainty of network connectivity and potential startup races.  The API design formalizes that behavior - it is expected that the components "heal" by waiting for their dependencies to become available.  Where possible, the host system will attempt to offer blocking behavior on a per unit basis that allows the logic of the system to be distributed.  In some cases, like TCP and HTTP proxy load balancing, those systems already have mechanisms to tolerate components that may not be started.


Disk Structure
--------------

Assumptions:

* Gear identifiers are hexadecimal 32 character strings (may be changed) and specified by the caller.  Random distribution of
  identifiers is important to prevent unbalanced search trees
* Ports are passed by the caller and are assumed to match to the image.  A caller is allowed to specify an external port,
  which may fail if the port is taken.
* Directories which hold ids are partitioned by integer blocks (ports) or the first two characters of the id (ids) to prevent
  gear sizes from growing excessively.
* The structure of persistent state on disk should facilitate administrators recovering the state of their systems using
  filesystem backups, and also be friendly to standard Linux toolchain introspection of their contents.


The on disk structure of geard is exploratory at the moment.  The major components are described below:

    /etc/systemd/system/container-active.target.wants/
      ctr-abcdef.service -> <symlink>

        This directory is read by systemd on startup (container-active.target is WantedBy multi-user) to 
        start containers on startup.  Containers stopped via the stop API call will not be started on
        reboot.

    /var/lib/containers/
      All content is located under this root

      units/
        ab/
          ctr-abcdef.service   # hardlink to the current unit file version
          ctr-abcdef.idle      # flag indicating this unit is currently idle
          abcdef/
            <requestid>        # a particular version of the unit file.

        A container is considered present on this system if a service file exists inside the namespaced container
        directory.

        The unit file is "enabled" in systemd (symlinked to systemd's unit directory) upon creation, and "disabled"
        (unsymlinked) on the remove operation.  The definition can be updated atomically (write new definition,
        update hardlink) when a new version of the container is deployed to the system.

        If a container is idled, a flag is written to the appropriate units directory.  Only containers with an
        idle flag are considered valid targets for unidling.

      targets/
        container.target         # default target
        container-active.target  # active target

        All containers are assigned to one of these two targets - on create or start, they have
        "WantedBy=container-active.target".  If a container is stopped via the API it is altered to be 
        "WantedBy=container.target".  In this fashion the disk structure for each unit reflects whether the container
        should be started on reboot vs. being explicitly idled.  Also, assuming the /var/lib/containers directory
        is an attached disk, on node recovery each *.service file is enabled with systemd and then the
        "container-active.target" can be started.

      slices/
        container.slice        # default slice
        container-small.slice  # more limited slice

        All slice units are created in this directory.  At the moment, the two slices are defaults and are created
        on first startup of the process, enabled, then started.  More advanced cgroup settings must be configured
        after creation, which is outside the scope of this prototype.

        All containers are created in the "container-small" slice at the moment.

      env/
        contents/
          a3/
            a3408aabfed

            Files storing environment variables and values in KEY="VALUE" (one per line) form.

      data/
        TBD (reserved for container unique volumes)

      ports/
        links/
          3f/
            3fabc98341ac3fe...24  # text file describing internal->external links to other networks

            Each container has one file with one line per network link, internal port first, a tab, then
            external port, then external host IP / DNS.

            On startup, gear init --post attempts to convert this file to a set of iptables rules in
            the container to outbound traffic.

        interfaces/
          1/
            49/
              4900  # softlink to the container's unit file

              To allocate a port, the daemon scans a block (49) of 100 ports for a set of free ports.  If no ports
              are found, it continues to the next block.  Currently the daemon starts at the low end of the port
              range and walks disk until it finds the first free port.  Worst case is that the daemon would do
              many directory reads (30-50) until it finds a gap.

              To remove a container, the unit file is deleted, and then any broken softlinks can be deleted.

              The first subdirectory represents an interface, to allow future expansion of the external IP space
              onto multiple devices, or to allow multiple external ports to be bound to the same IP (for VPC)

              Example script:

                sudo find /var/lib/containers/ports/interfaces -type l -printf "%l %f " -exec cut -f 1-2 {} \;

              prints the port description path (of which the name of the path is the container id), the public port,
              and the value of the description file (which might have multiple lines).  Would show what ports
              are mismatched.

      keys/
        ab/
          ab0a8oeunthxjqkgjfrJQKNHa7384  # text file in authorized_keys format representing a single public key

          Each file represents a single public key, with the identifier being the a base64 encoded SHA256 sum of
          the binary value of the key.  The file is stored in authorized_keys format for SSHD, but with only the
          type and value sections present and no newlines.

          Any key that has zero incoming links can be deleted.

      access/
        containers/
          3f/
            3fabc98341ac3fe...24/  # container id
              key1  # softlink to a public key authorized to access this container

              The names of the softlink should map to an container id or container label (future) - each container id should match
              to a user on the system to allow sshd to login via the container id.  In the future, improvements in sshd
              may allow us to use virtual users.

        git/
          read/
            ab/
              ab934xrcgqkou08/  # repository id
                key1  # softlink to a public key authorized for read access to this repo

          write/
            ab/
              ab934xrcgqkou08/  # repository id
                key2  # softlink to a public key authorized for write access to this repo

Building Images
---------------

geard uses [Docker Source to Images (STI)](http://github.com/openshift/docker-source-to-images)
to build deployable images from a base image and application source.  STI supports a number of
use cases for building deployable images, including:

1. Use a git repository as a source
1. Incremental builds: downloaded dependencies and generated artifacts are re-used across builds
1. Extended prepare: build and deploy on different images (compatible with incremental builds)

A number of public STI base images exist:

1. `pmorie/centos-ruby2` - ruby2 on centos
1. `pmorie/ubuntu-buildpack` - foreman running on ubuntu
1. `pmorie/fedora-mock` - a simple Webrick server for static html, on fedora

See the STI docs for information on creating your own base images to use with STI.

License
-------

Apache Software License (ASL) 2.0.

