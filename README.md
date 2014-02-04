geard
=====

Exploration of a Docker+systemd gear daemon (geard) in Go.  The daemon runs at a system level and interfaces with the Docker daemon and systemd over DBus to install and run Docker containers on the system.  It will also support retrieval of local content for use in a distributed environment.


Try it out
----------

Take the systemd unit file in <code>contrib/geard.service</code> and enable it on your system (assumes Docker is installed) with: 

    systemctl enable <path_to_geard.service>
    mkdir -p /var/lib/gears/units
    systemctl start geard
    
The first time it executes it'll download the latest Docker image for geard which may take a few minutes.  After it's started, make the following curl call:

    curl -X PUT "http://localhost:2223/token/__test__/containers?u=deadbeef&d=1&t=pmorie%2Fsti-html-app&r=1&i=1" -d '{}'
    
This will install a new systemd unit to <code>/var/lib/gears/units/gear-1.service</code> and invoke start.  Use

    systemctl status gear-1
    
to see whether the startup succeeded.

NOTE: the <code>i</code> parameter is a unique request ID - geard will filter duplicate requests, so if you change the request parameters be sure to increment <code>i</code> to a higher hexadecimal number (2, a, 2a, etc).

License
-------

Apache Software License (ASL) 2.0.

