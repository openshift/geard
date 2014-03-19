#!/bin/bash
echo "Initializing gear daemon"
docker pull ccoleman/geard
docker pull kraman/githost
docker pull pmorie/sti-html-app
systemctl start /usr/lib/systemd/system/geard.service

echo "stop and & disable geard.service if you are compiling it locally"
echo "systemctl stop /usr/lib/systemd/system/geard.service"
echo "systemctl disable /usr/lib/systemd/system/geard.service"
