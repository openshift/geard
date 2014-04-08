#!/bin/bash
echo "Setting up vm for broker development"

# GEARD_PATH
GEARD_DIR=/vagrant/src/github.com/openshift/geard
SCRIPT_DIR=$GEARD_DIR/contrib
IMAGE_MANIFESTS=$SCRIPT_DIR/images_manifests
ORIGIN_SERVER=/openshift/origin-server
MONGO_IMAGE=10.64.27.125:5000/mongo/mongdb_auto_vol_standalone

echo "Build origin-server docker image"
docker build -rm -t openshift-origin-broker $ORIGIN_SERVER/.

echo "Pull MongoDB RHEL image"
docker pull $MONGO_IMAGE

echo "Starting MongoDB"
docker run -name mongodb -d -p 27017:27017 $MONGO_IMAGE

echo "Starting Openshift Origin Broker"
docker run -name broker -d -i -t -p 3000:443 -e "MONGO_HOST_PORT=172.17.0.2:27017" -v $ORIGIN_SERVER:/var/www/openshift/ openshift-origin-broker

# sleep to let bundle install complete before proceeding
sleep 10 

echo "Populating Openshift Origin Broker Mongo Prereqs"
docker run -rm -i -e "MONGO_HOST_PORT=172.17.0.2:27017" -v $SCRIPT_DIR:/root/scripts -v $IMAGE_MANIFESTS:/root/cartridge_image_manifests -v $ORIGIN_SERVER:/var/www/openshift/ openshift-origin-broker /bin/bash --login /root/scripts/bootstrap-broker-mongo.sh

echo "To interact with broker use normal rhc commands"
echo "To setup RHC tools, run following:"
echo "  yum install -y rubygem-rhc"
echo "  rhc setup --clean --server http://localhost:3000"