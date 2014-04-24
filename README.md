geard [![Build Status](https://travis-ci.org/openshift/geard.png?branch=master)](https://travis-ci.org/openshift/geard)
=====

geard is a command line client for installing [Docker](https://www.docker.io) images as containers onto a systemd-enabled Linux operating system (systemd 207 or newer).  It may be run as a command:

    $ sudo gear install pmorie/sti-html-app my-sample-service

to install the public image <code>pmorie/sti-html-app</code> to systemd on the local machine with the service name "ctr-my-sample-service".  The command can also start as a daemon and serve API requests over HTTP (default port 43273) :

    $ sudo gear daemon
    2014/02/21 02:59:42 ports: searching block 41, 4000-4099
    2014/02/21 02:59:42 Starting HTTP on :43273 ...

The `gear` CLI can connect to this agent:

    $ gear stop localhost/my-sample-service
    $ gear install pmorie/sti-html-app localhost/my-sample-service.1 localhost/my-sample-service.2
    $ gear start localhost/my-sample-service.1 localhost/my-sample-service.2

The geard agent exposes operations on containers needed for [large scale orchestration](./docs/orchestrating_geard.md) in production environments, and tries to map those operations closely to the underlying concepts in Docker and systemd.  It supports linking containers into logical groups (applications) across multiple hosts with [iptables based local networking](./docs/linking.md), shared environment files, and SSH access to containers.  It is also a test bed for prototyping related container services that may eventually exist as Docker plugins, such as routing, event notification, and efficient idling and network activation.

The gear daemon and local commands must run as root to interface with the Docker daemon over its Unix socket and systemd over DBus.


### What is a "gear", and why Docker?

In OpenShift Origin, a gear is a secure, isolated environment for user processes using cGroups and SELinux.  As Linux namespace technology has evolved to provide other means of constraining processes, the term "container" has become prevalent, and is used interchangeably below.  Docker has made the creation and distribution of container images effortless, and the ability to reproducibly run a Linux application in many environments is a key component for developers and administrators.  At the same time, the systemd process manager has unified many important Linux process subsystems (logging, audit, managing and monitoring processes, and controlling cGroups) into a reliable and consistent whole.


### What are the key requirements for production containers?

* **Containers are securely isolated from the host except through clear interfaces**

  By default, a container should only see what the host allows - being able to become root within a container is extremely valuable for installing packaged software, but that is also a significant security concern.  Both user namespaces and SELinux are key components to protecting the host from arbitrary code, and should be secure by default within Docker.  However, as necessary administrators should be able to expose system services or other containers to a container.  Other limits include network abstractions and quota restrictions on the files containers create.

* **Container processes should be independent and resilient to failure**

  Processes fail, become corrupted, and die.  Those failures should be isolated and recoverable - a key feature of systemd is its comprehensive ability to handle the wide variety of process death and restart, recover, limit, and track the involved processes.  The failure of other components within the system should not block restarting or reinitializing other containers to the extent possible, especially in bulk.

* **Containers should be portable across hosts**

  A Docker image should be reusable across hosts.  This means that the underlying Docker abstractions (links, port mappings, environment files) should be used to ensure the gear does not become dependent on the host system except where necessary.  The system should make it easy to share environment and context between gears and move or recreate them among host systems.

* **Containers must be auditable, constrained, and reliably logged**

  Many of the most important characteristics of Linux security are difficult to enforce on arbitrary processes. systemd provides standard patterns for each of these and when properly integrated with Docker can give administrators in multi-tenant or restricted environments peace of mind.


### Actions on a container

Here are the supported container actions on the agent - these should map cleanly to Docker, systemd, or a very simple combination of the two.  Extensions are intended to simplify cross container actions (shared environment and links)

*   Create a new system unit file that runs a single docker image (install and start a container)

        $ gear install pmorie/sti-html-app localhost/my-sample-service --start -p 8080:0

        $ curl -X PUT "http://localhost:43273/container/my-sample-service" -H "Content-Type: application/json" -d '{"Image": "pmorie/sti-html-app", "Started":true, "Ports":[{"Internal":8080}]}'

*   Stop, start, and restart a container

        $ gear stop localhost/my-sample-service
        $ gear start localhost/my-sample-service
        $ gear restart localhost/my-sample-service

        $ curl -X PUT "http://localhost:43273/container/my-sample-service/stopped"
        $ curl -X PUT "http://localhost:43273/container/my-sample-service/started"
        $ curl -X POST "http://localhost:43273/container/my-sample-service/restart"

*   Deploy a set of containers on one or more systems, with links between them:

        # create a simple two container web app
        $ gear deploy deployment/fixtures/simple_deploy.json localhost

    Deploy creates links between the containers with iptables - use nsenter to join the container web-1 and try curling 127.0.0.1:8081 to connect to the second web container.  These links are stable across hosts and can be changed without the container knowing.

        # create a mongo db replica set (some assembly required)
        $ gear deploy deployment/fixtures/mongo_deploy.json localhost
        $ sudo switchns --container=db-1 -- /bin/bash
        > mongo 192.168.1.1
        MongoDB shell version: 2.4.9
        > rs.initiate({_id: "replica0", version: 1, members:[{_id: 0, host:"192.168.1.1:27017"}]})
        > rs.add("192.168.1.2")
        > rs.add("192.168.1.3")
        > rs.status()
        # wait....
        > rs.status()

    Note: The argument to initiate() sets the correct hostname for the first member, otherwise the other members cannot connect.

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

*   [Link containers](./docs/linking.md) with local loopback ports (for e.g. 127.0.0.2:8081 -> 9.8.23.14:8080). If local ip isn't specified, it defaults to 127.0.0.1

        $ gear link -n=127.0.0.2:8081:9.8.23.14:8080 localhost/my-sample-service

*   Set a public key as enabling SSH or Git SSH access to a container or repository (respectively)

        $ gear keys --key-file=[FILE] my-sample-service
        $ curl -X POST "http://localhost:43273/keys" -H "Content-Type: application/json" -d '{"Keys": [{"Type":"authorized_keys","Value":"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ=="}], "Containers": [{"Id": "my-sample-service"}]}'

*   Enable SSH access to join a container for a set of authorized keys

        TODO: add examples and make setup cleaner

*   Build a new image using [Docker Source-to-Images](https://github.com/openshift/docker-source-to-images) from a source URL and base image

        # build an image on the local system and tag it as mybuild-1
        $ gear build git://github.com/pmorie/simple-html pmorie/fedora-mock mybuild-1

        # remote build
        $ curl -X POST "http://localhost:43273/build-image" -H "Content-Type: application/json" -d '{"BaseImage":"pmorie/fedora-mock","Source":"git://github.com/pmorie/simple-html","Tag":"mybuild-1"}'

*   Create a Git repository on the host

        $ gear create-repo localhost/my-sample-repo [<optional source url to clone>]

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

geard allows an administrator to easily ensure a given Docker container will *always* run on the system by creating a systemd unit describing a docker run command.  It will execute the Docker container processes as children of the systemd unit, allowing auto restart of the container, customization of additional namespace options, the capture stdout and stderr to journald, and audit/seccomp integration to those child processes.  Note that foreground execution is currently not in Docker master - see https://github.com/alexlarsson/docker/tree/forking-run for some prototype work demonstrating the concept.

Each created systemd unit can be assigned a unique Unix user for quota and security purposes with the `--isolate` flag, which prototypes isolation prior to user namespaces being part of Docker.  An SELinux MCS category label will automatically be assigned to the container to separate it from the other containers on the system, and containers can be set into systemd slices with resource constraints.


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

`vagrant up` will install a few RPMs the first time it is started.  Once `vagrant up` is running, you can ssh into the vm:

    vagrant ssh

The `contrib/build` script checks and downloads Go dependencies, builds the `gear` binary, and then installs it to /vagrant/bin/gear and /usr/bin/gear.  It has a few flags - '-s' builds with SELinux support for SSH and Git.

    contrib/build -s

Once you've built the executables, you can run:

    sudo $GOPATH/bin/gear daemon

to start the gear agent.  The agent will listen on port 43273 by default and print logs to the console - hit CTRL+C to stop the agent.

See [contrib/example.sh](contrib/example.sh) and [contrib/stress.sh](contrib/stress.sh) for more examples of API calls.

An example systemd unit file for geard is included in the `contrib/` directory.  After building, the following commands will install the unit file and start the agent under systemd:

    sudo systemctl enable contrib/geard.service
    sudo systemctl start contrib/geard.service


How can geard be used in orchestration?
---------------------------------------

[See the orchestrating geard doc](./docs/orchestrating_geard.md)


API Design
----------

[See the API design doc](./docs/api_design.md)


Disk Structure
--------------

[Description of storage on disk](./docs/disk_structure.md)


geard Concepts
--------------

Outline of how some of the core operations work:

* [Linking](./docs/linking.md) - use iptable rules and environment variables to simplify container interconnect
* SSH - generate authorized_keys for a user on demand
* Isolated container - start an arbitrary image and force it to run as a given user on the host by chown the image prior to execution
* Idling - use iptable rules to wake containers on SYN packets
* Git - host Git repositories inside a running Docker container
* Logs - stream journald log entries to clients
* Builds - use transient systemd units to execute a build inside a container
* Jobs - run one-off jobs as systemd transient units and extract their logs and output after completion

Not yet prototyped:

* Integrated health check - mark containers as available once a pluggable/configurable health check passes
* Joining - reconnect to an already running operation
* Direct server to server image pulls - allow hosts to act as a distributed registry
* Job callbacks - invoke a remote endpoint after an operation completes
* Local routing - automatically distribute config for inbound and outbound proxying via HAProxy
* Repair - cleanup and perform consistency checks on stored data (most operations assume some cleanup)
* Capacity reporting - report capacity via API calls, allow precondition PUTs based on remaining capacity ("If-Match: capacity>=5"), allow capacity to be defined via config


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

