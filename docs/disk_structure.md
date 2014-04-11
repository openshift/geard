Disk Structure
--------------

Assumptions:

* Identifiers are 24 character strings (subject to change) and specified by the caller.  Random distribution of
  identifiers is important to prevent unbalanced search trees in large deployments - but in small deployments descriptive 
  names are just as valuable.
* Ports are passed by the caller and are assumed to match to the image.  A caller is allowed to specify an external port,
  which may fail if the port is taken.
* To prevent directories from reach excessive size at high density, directories mapped to ids are partitioned by the first 
  two characters of the id or, for ports a modulus value.
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
