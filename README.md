geard
=====

Exploration of a Docker+systemd gear daemon (geard) in Go.  The daemon runs at a system level and interfaces with the Docker daemon and systemd over DBus to install and run Docker containers on the system.  It will also support retrieval of local content for use in a distributed environment.


Try it out
----------

Take the systemd unit file in <code>contrib/geard.service</code> and enable it on your system (assumes Docker is installed) with: 

    curl https://raw.github.com/smarterclayton/geard/master/contrib/geard.service > /usr/lib/systemd/service/geard.service
    systemctl enable /usr/lib/systemd/service/geard.service
    mkdir -p /var/lib/gears/units
    systemctl start geard
    
The first time it executes it'll download the latest Docker image for geard which may take a few minutes.  After it's started, make the following curl call:

    curl -X PUT "http://localhost:2223/token/__test__/container?u=0&d=1&t=pmorie%2Fsti-html-app&r=1&i=1" -d '{"ports":[{"external":"4343","internal":"8080"}]}'
    
This will install a new systemd unit to <code>/var/lib/gears/units/gear-1.service</code> and invoke start, and expose the port 8080 at 4343 on the host.  Use

    systemctl status gear-1
    
to see whether the startup succeeded.

To start that gear, run:

    curl -X PUT "http://localhost:2223/token/__test__/container/started?u=0&d=1&r=1&i=2"

and to stop run:

    curl -X PUT "http://localhost:2223/token/__test__/container/stopped?u=0&d=1&r=1&i=2"

NOTE: the <code>i</code> parameter is a unique request ID - geard will filter duplicate requests, so if you change the request parameters be sure to increment <code>i</code> to a higher hexadecimal number (2, a, 2a, etc).


License
-------

Apache Software License (ASL) 2.0.

