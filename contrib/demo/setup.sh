#!/bin/sh -x

base=$(dirname $0)

id=$(docker inspect --format="{{.id}}" openshift/rhel-mongodb-repl)
ret=$?
if [ $ret -ne 0 ] || [ "$FETCH_IMAGES" != "" ]; then
  docker pull openshift/geard-githost
  docker pull docker-registry1.dev.rhcloud.com/jboss/eap
  docker tag  docker-registry1.dev.rhcloud.com/jboss/eap jboss/eap
  docker pull docker-registry1.dev.rhcloud.com/ccoleman/eap-scaling-demo
  docker tag  docker-registry1.dev.rhcloud.com/ccoleman/eap-scaling-demo openshift/demo-eap-scaling
  docker pull docker-registry1.dev.rhcloud.com/openshift/rhel-mongodb
  docker tag  docker-registry1.dev.rhcloud.com/openshift/rhel-mongodb openshift/rhel-mongodb
  docker pull docker-registry1.dev.rhcloud.com/openshift/rhel-mongodb-repl
  docker tag  docker-registry1.dev.rhcloud.com/openshift/rhel-mongodb-repl openshift/rhel-mongodb-repl
  docker pull docker-registry1.dev.rhcloud.com/openshift/demo-ews
  docker tag  docker-registry1.dev.rhcloud.com/openshift/demo-ews openshift/demo-ews

  docker pull pmorie/sti-html-app
  docker pull openshift/openshift-broker-docker
  docker tag  103bd59de294 jboss/eap # tag is in the history
fi

set +x

units=$(curl -q http://localhost:43273/containers)
ret=$?
if [ $ret -ne 0]; then
  echo "gear daemon not responding, make sure the service is running and retry."
fi

$base/teardown.sh

gear deploy $base/deploy_eap_cluster.json localhost
sleep 3
gear stop localhost/demo-backend-3

$base/wait_for_url.sh "http://localhost:14000/scale-1.0/"
