#!/bin/bash
echo "Initializing gear daemon"

echo "To use the geard service, run"
echo "systemctl enable /usr/lib/systemd/system/geard.service"
echo "systemctl start geard"
echo ""
echo "Otherwise, run"
echo "vagrant ssh"
echo "contrib/build"
