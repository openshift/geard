geard
=====

Exploration of a Docker+systemd gear daemon (geard) in Go.  The daemon runs at a system level and interfaces with the Docker daemon and systemd over DBus to install and run Docker containers on the system.  It will also support retrieval of local content for use in a distributed environment.


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
    
The first time it executes it'll download the latest Docker image for geard which may take a few minutes.  After it's started, make the following curl call:

    curl -X PUT "http://localhost:2223/token/__test__/container?u=0&d=1&t=pmorie%2Fsti-html-app&r=1111&i=1" -d '{"ports":[{"external":"4343","internal":"8080"}]}'
    
This will install a new systemd unit to <code>/var/lib/gears/units/gear-1.service</code> and invoke start, and expose the port 8080 at 4343 on the host.  Use

    systemctl status gear-1
    
to see whether the startup succeeded.  If you set the external port to 0 or omit the field, geard will allocate a port for you between 4000 and 60000.

A brief note: the /token/__test__ prefix (and the r, t, u, d, and i parameters) are intended to be replaced with a cryptographic token embedded in the URL path.  This will allow the daemon to take an encrypted token from a known public key and decrypt and verify authenticity.  For testing purposes the __test__ token allows the keys inside the proposed token to be passed as request parameters instead.

To start that gear, run:

    curl -X PUT "http://localhost:2223/token/__test__/container/started?u=0&d=1&r=1111&i=2"

and to stop run:

    curl -X PUT "http://localhost:2223/token/__test__/container/stopped?u=0&d=1&r=1111&i=3"

To stream the logs from the gear over http, run:

    curl -X GET "http://localhost:2223/token/__test__/container/log?u=0&d=1&r=1111&i=4"

The logs will close after 30 seconds.

To create a new repository, ensure the /var/lib/gears/git directory is created and then run:

    curl -X PUT "http://localhost:2223/token/__test__/repository?u=0&d=1&r=git1&i=5"

First creation will be slow while the ccoleman/githost image is pulled down.  Repository creation will use a systemd transient unit named <code>job-&lt;r&gt;</code> - to see status run:

    systemctl status job-git1

If you want to create a repository based on a source URL, pass <code>t=&lt;url&gt;</code> to the PUT repository call.  Once you've created a repository with at least one commit, you can stream a git archive zip file of the contents with:

    curl "http://localhost:2223/token/__test__/content?u=0&d=1&t=gitarchive&r=git2&i=7"

NOTE: the <code>i</code> parameter is a unique request ID - geard will filter duplicate requests, so if you change the request parameters be sure to increment <code>i</code> to a higher hexadecimal number (2, a, 2a, etc).


Disk Structure
--------------

Assumptions:

* Gear identifiers are hexadecimal 32 character strings (may be changed) and specified by the caller.  Even distribution of
  identifiers is important to prevent unbalanced search trees
* Ports are passed by the caller and are assumed to match to the image.  A caller is allowed to specify an external port,
  which may fail if the port is taken.
* Directories which hold ids are partitioned by integer blocks (ports) or the first two characters of the id (ids) to prevent
  gear sizes from growing excessively.

The on disk structure of geard is exploratory at the moment.

    /var/lib/gears/
      All content is located under this root

      units/
        gear-<gearid>.service  # systemd unit file

        A gear is considered present on this system if a service file exists in this directory.  Creation attempts
        to exclusively create this file and will fail otherwise.

        The unit file is "enabled" in systemd (symlinked to systemd's unit directory) upon creation, and "disabled"
        (unsymlinked) on the stop operation.  Disabled gears won't be started on restart - this is equivalent to
        "I don't want you to start this again".

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
            3fabc98341ac3fe...24  # softlink to a key

            The names of the softlink should map to an gear id or gear label (future) - each gear id should match
            to a user on the system to allow sshd to login via the gear id.  In the future, improvements in sshd
            may allow us to use virtual users.

        git/
          read/

          write/

License
-------

Apache Software License (ASL) 2.0.

