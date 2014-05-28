#!/bin/sh -x

base=$(dirname $0)

id=$(docker inspect --format="{{.id}}" openshift/centos-haproxy-simple-balancer)
ret=$?
if [ $ret -ne 0 ] || [ "$FETCH_IMAGES" != "" ]; then
  docker pull openshift/centos-haproxy-simple-balancer
  docker pull openshift/nodejs-0-10-centos
  docker tag openshift/nodejs-0-10-centos nodejs-centos
  docker pull openshift/centos-mongodb
  docker pull pmorie/parks-map-app
  docker tag pmorie/parks-map-app parks-map-app
fi

set +x

units=$(curl -q http://localhost:43273/containers)
ret=$?
if [ $ret -ne 0 ]; then
  echo "gear daemon not responding, make sure the service is running and retry."
  exit 1
fi

$base/teardown.sh

descriptor=$base/deploy_parks_map.json
if [ "$MULTIHOST" != "" ]; then
  descriptor=$base/deploy_parks_map_instances.json
fi

gear deploy $descriptor localhost
gear stop localhost/parks-backend-{2,3}

$base/wait_for_url.sh "http://localhost:14000/"

sudo switchns --container=parks-db-1 -- /bin/bash -c "curl https://raw.githubusercontent.com/thesteve0/fluentwebmap/master/parkcoord.json | mongoimport -d fluent -c parkpoints --type json && mongo fluent --eval 'db.parkpoints.ensureIndex( { pos : \"2d\" } );'"
