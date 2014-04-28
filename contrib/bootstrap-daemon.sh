#!/bin/bash
echo "Initializing gear daemon"

echo "To use the geard service, run"
echo "sudo systemctl enable $GOPATH/src/github.com/openshift/geard/contrib/geard.service"
echo "sudo systemctl start geard.service"
echo ""
echo "To use the geard-githost service, run"
echo "systemctl enable /var/lib/containers/units/geard-githost.service"
echo "systemctl start geard-githost.service"
echo ""
echo "Otherwise, run"
echo "vagrant ssh"
echo "contrib/build"
