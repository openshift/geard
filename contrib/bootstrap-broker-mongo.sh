#!/bin/bash
echo "Import cartridges"
find /root/cartridge_image_manifests -iname manifest.yml | oo-admin-ctl-cartridge -c import --activate

echo "Populate geard node location"
/var/www/openshift/broker-util/oo-admin-ctl-node -c create --name default --server_identity 172.17.42.1:43273