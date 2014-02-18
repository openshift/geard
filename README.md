geard [![Build Status](https://travis-ci.org/smarterclayton/geard.png?branch=master)](https://travis-ci.org/smarterclayton/geard)
=====

Exploration of a Docker+systemd gear daemon (geard) in Go.  The daemon runs at a system level and interfaces with the Docker daemon and systemd over DBus to install and run Docker containers on the system.  It will also support retrieval of local content for use in a distributed environment.

The primary operations are:

* Create a new system unit file that runs a single docker image (create and run a gear)
* Stop or start a gear
* Fetch the logs for a gear
* Create a new empty Git repository
* Set a public key as enabling SSH or Git SSH access to a gear or repository (respectively)
* Build a new image from a source URL and base image
* Fetch a Git archive zip for a repository
* Set and retrieve environment files for sharing between gears (patch and pull operations)


Try it out
----------

The geard code depends on:

* systemd 207 (Fedora 20 or newer)
* Docker 0.7 or newer

You can get a vagrant F20 box from http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_fedora-20_chef-provisionerless.box

Take the systemd unit file in <code>contrib/geard.service</code> and enable it on your system (assumes Docker is installed) with: 

    curl https://raw.github.com/smarterclayton/geard/master/contrib/geard.service > /usr/lib/systemd/service/geard.service
    systemctl enable /usr/lib/systemd/service/geard.service
    mkdir -p /var/lib/gears/units
    systemctl start geard

The service is set to bind to port 2223 and is accessible on localhost.
    
The first time it executes it'll download the latest Docker image for geard which may take a few minutes.  After it's started, make the following curl call:

    curl -X PUT "http://localhost:2223/token/__test__/container?u=0&d=1&t=pmorie%2Fsti-html-app&r=0001&i=1" -d '{"ports":[{"external":"4343","internal":"8080"}]}'
    
This will install a new systemd unit to <code>/var/lib/gears/units/gear-0001.service</code> and invoke start, and expose the port 8080 at 4343 on the host.  Use

    systemctl status gear-0001
    
to see whether the startup succeeded.  If you set the external port to 0 or omit the field, geard will allocate a port for you between 4000 and 60000.

A brief note: the /token/__test__ prefix (and the r, t, u, d, and i parameters) are intended to be replaced with a cryptographic token embedded in the URL path.  This will allow the daemon to take an encrypted token from a known public key and decrypt and verify authenticity.  For testing purposes the __test__ token allows the keys inside the proposed token to be passed as request parameters instead.

To start that gear, run:

    curl -X PUT "http://localhost:2223/token/__test__/container/started?u=0&d=1&r=0001&i=2"

and to stop run:

    curl -X PUT "http://localhost:2223/token/__test__/container/stopped?u=0&d=1&r=0001&i=3"

To stream the logs from the gear over http, run:

    curl -X GET "http://localhost:2223/token/__test__/container/log?u=0&d=1&r=0001&i=4"

The logs will close after 30 seconds.

To create a new repository, ensure the /var/lib/gears/git directory is created and then run:

    curl -X PUT "http://localhost:2223/token/__test__/repository?u=0&d=1&r=git1&i=5"

First creation will be slow while the ccoleman/githost image is pulled down.  Repository creation will use a systemd transient unit named <code>job-&lt;r&gt;</code> - to see status run:

    systemctl status job-git1

If you want to create a repository based on a source URL, pass <code>t=&lt;url&gt;</code> to the PUT repository call.  Once you've created a repository with at least one commit, you can stream a git archive zip file of the contents with:

    curl "http://localhost:2223/token/__test__/content?u=0&d=1&t=gitarchive&r=git2&i=7"

To set an environment file:

    curl -X PUT "http://localhost:2223/token/__test__/environment?u=0&d=1&r=1000&i=8" -d '{"env":[{"name":"foo","value":"bar"}]}'

and to retrieve that environment (in normalized env file form)

    curl "http://localhost:2223/token/__test__/content?u=0&d=1&t=env&r=1000&i=9"

NOTE: the <code>i</code> parameter is a unique request ID - geard will filter duplicate requests, so if you change the request parameters be sure to increment <code>i</code> to a higher hexadecimal number (2, a, 2a, etc).

See [contrib/example.sh](contrib/example.sh) and [contrib/stress.sh](contrib/stress.sh) for more examples of API calls.


API Design
----------

The API is structured around fast and slow idempotent operations - all API responses should finish their primary objective in <10ms with success or failure, and either return immediately with failure, or on success additional data may be streamed to the client in structured (JSON events) or unstructured (logs from journald) form.  In general, all operations should be reentrant - invoking the same operation multiple times with different request ids should yield exactly the same result.  Some operations cannot be repeated because they depend on the state of external resources at a point in time (build of the "master" branch of a git repository) and subsequent operations may not have the same outcome.  These operations should be gated by request identifier where possible, and it is the client's responsibility to ensure that condition holds.

The API takes into account the concept of "joining" - if two requests are made with the same request id, where possible the second request should attach to the first job's result and streams in order to provide an identical return value and logs.  This allows clients to design around retries or at-least-once delivery mechanisms safely.  The second job may check the invariants of the first as long as data races can be avoided.

All non-content-streaming jobs (which should already be idempotent and repeatable) will eventually be structured in two phases - execute and report.  The execute phase attempts to assert that the state on the host is accurate (systemd unit created, symlinks on disk, data input correct) and to return a 2xx response on success or an error body and 4xx or 5xx response on error as fast as possible.  API operations should *not* wait for asynchronous events like the stable start status of a process, the external ports being bound, or image specific data to be written to disk.  Instead, those are modelled with separate API calls.  The report phase is optional for all jobs, and is where additional data may be streamed to the consumer over HTTP or a message bus.

In general, the philosophy of create/fail fast operations is based around the recognition that distributed systems may fail at any time, but those failures are rare.  If a failure does occur, the recovery path is for a client to retry the operation as originally submitted, or to delete the affected resources, or in rare cases for the system to autocorrect.  A service may take several minutes to start only to fail - since failure cannot be predicted, clients should be given tools to recognize and correct failures.


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

    /var/lib/gears/
      All content is located under this root

      units/
        gear-<gearid>.service  # systemd unit file

        A gear is considered present on this system if a service file exists in this directory.  Creation attempts
        to exclusively create this file and will fail otherwise.

        The unit file is "enabled" in systemd (symlinked to systemd's unit directory) upon creation, and "disabled"
        (unsymlinked) on the stop operation.  Disabled services are not automatically started on reboot.  The "start"
        operation against a gear will reenable the service.

      slices/
        gear.slice        # default slice
        gear-small.slice  # more limited slice

        All slice units are created in this directory.  At the moment, the two slices are defaults and are created
        on first startup of the process, enabled, then started.  More advanced cgroup settings must be configured
        after creation, which is outside the scope of this prototype.

        All containers are created in the "gear-small" slice at the moment.

      data/
        TBD (reserved for gear unique volumes)

      ports/
        descriptions/
          3f/
            3fabc98341ac3fe...24  # text file describing internal->external ports

            Each entry is a text file with one line per port pair, internal port first, a tab, then external port.
            These files are symlinked from the interfaces directory.

            A gear that has no port description file should have no ports exposed.

        interfaces/
          1/
            49/
              4900  # softlink to a port description file for a gear

              To allocate a port, the daemon scans a block (49) of 100 ports for a set of free ports.  If no ports
              are found, it continues to the next block.  Currently the daemon starts at the low end of the port
              range and walks disk until it finds the first free port.  Worst case is that the daemon would do
              many directory reads (30-50) until it finds a gap.

              To remove a gear, the port description is deleted, and then any broken softlinks can be deleted.

              The first subdirectory represents an interface, to allow future expansion of the external IP space
              onto multiple devices, or to allow multiple external ports to be bound to the same IP (for VPC)

              Example script:

                sudo find /var/lib/gears/ports/interfaces -type l -printf "%l %f " -exec cut -f 1-2 {} \;

              prints the port description path (of which the name of the path is the gear id), the public port,
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
        gears/
          3f/
            3fabc98341ac3fe...24/  # gear id
              key1  # softlink to a public key authorized to access this gear

              The names of the softlink should map to an gear id or gear label (future) - each gear id should match
              to a user on the system to allow sshd to login via the gear id.  In the future, improvements in sshd
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

License
-------

Apache Software License (ASL) 2.0.

