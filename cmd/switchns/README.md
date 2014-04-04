geard-switchns
==============

Utility command which switches into a specified docker container's namespace and execute a command.
It allows two use-cases:

* Admin commands

Usage:

Usage:
	switchns --container=<container name> [--env="key=value"]... [--] <command>...
	
Examples:
	switchns --container=gear-0001 -- /bin/echo 1
	switchns --container=gear-0001 -- /bin/bash -c "echo \$PATH"
	switchns --container=gear-0001 --env="FOO=BAR" --env="BAZ=ZAB" -- /bin/bash -c "echo \$FOO \$BAZ"
        
Allows a user with CAP_SYS_ADMIN capability to switch into a specified docker container and execute a command.
Typical use for this would be to run admin commands within a container.

* User SSH

Add geard-switchns as a command to the ```.authorized_keys``` file. Eg:

    command="/usr/sbin/switchns" ssh-rsa AAAA...== user@host

When the user SSH's into the host machine, SSH runs ```geard-switchns```. The utility then looks up a docker container
with the same name as the username and starts a bash shell within the container.

License
=======

Apache Software License (ASL) 2.0.
