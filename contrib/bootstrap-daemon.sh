#!/bin/bash
echo "Initializing gear daemon"

echo "To use the geard service, run"
echo "systemctl enable /usr/lib/systemd/system/geard-image.service"
echo "systemctl start geard-image.service"
echo ""
echo "To use the geard-githost service, run"
echo "systemctl enable /var/lib/containers/units/geard-githost.service"
echo "systemctl start geard-githost.service"
echo ""
echo "Otherwise, run"
echo "vagrant ssh"
echo "contrib/build"
