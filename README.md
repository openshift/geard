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

A brief note: the /token/__test__ prefix (and the r, t, u, d, and i parameters) are intended to be replaced with a cryptographic token embedded in the URL path.  This will allow the daemon to take an encrypted token from a known public key and decrypt and verify authenticity.  For testing purposes the __test__ token allows the keys inside the proposed token to be passed as request parameters instead.

To start that gear, run:

    curl -X PUT "http://localhost:2223/token/__test__/container/started?u=0&d=1&r=1&i=2"

and to stop run:

    curl -X PUT "http://localhost:2223/token/__test__/container/stopped?u=0&d=1&r=1&i=3"

To stream the logs from the gear over http, run:

    curl -X PUT "http://localhost:2223/token/__test__/container/log?u=0&d=1&r=1&i=4"

The logs will close after 30 seconds.

To create a new repository, ensure the /var/lib/gears/git directory is created and then run:

    curl -X PUT "http://localhost:2223/token/__test__/repository?u=0&d=1&r=git1&i=5"

First creation will be slow while the ccoleman/githost image is pulled down.  Repository creation will use a systemd transient unit named <code>job-&lt;r&gt;</code> - to see status run:

    systemctl status job-git1

If you want to create a repository based on a source URL, pass <code>t=&lt;url&gt;</code> to the PUT repository call.  Once you've created a repository with at least one commit, you can stream a git archive zip file of the contents with:

    curl "http://localhost:2223/token/__test__/content?u=0&d=1&t=gitarchive&r=git2&i=7"

NOTE: the <code>i</code> parameter is a unique request ID - geard will filter duplicate requests, so if you change the request parameters be sure to increment <code>i</code> to a higher hexadecimal number (2, a, 2a, etc).


License
-------

Apache Software License (ASL) 2.0.

