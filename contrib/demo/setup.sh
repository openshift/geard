#!/bin/sh -x

base=$(dirname $0)

id=$(docker inspect --format="{{.id}}" goldmann/mongod)
ret=$?
if [ $ret -ne 0 ]; then
  docker pull ccoleman/geard-githost
  docker pull 10.10.188.116/jboss/eap
  docker tag 10.10.188.116/jboss/eap jboss/eap
  docker pull 10.10.188.116/ccoleman/eap-scaling-demo
  docker tag 10.10.188.116/ccoleman/eap-scaling-demo ccoleman/eap-scaling-demo
  docker pull 10.10.188.116/ccoleman/eap-scaling-demo-new
  docker tag 10.10.188.116/ccoleman/eap-scaling-demo-new ccoleman/eap-scaling-demo-new
  docker pull 10.64.27.125:5000/goldmann/mongod
  docker tag 10.64.27.125:5000/goldmann/mongod goldmann/mongod
  docker pull pmorie/sti-html-app
  docker pull ccoleman/ubuntu-mongodb-repl
  docker pull ccoleman/openshift-broker-docker
  docker pull dockerfile/mongodb
  docker tag 103bd59de294 jboss/eap # tag is in the history
fi

set +x

$base/teardown.sh

gear deploy $base/deploy_eap_cluster.json localhost
sleep 3
gear stop localhost/demo-backend-3

$base/wait_for_url.sh "http://localhost:14000/scale-1.0/"