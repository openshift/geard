#!/bin/sh

base=$(dirname $0)

trap "echo" SIGINT

function run() {
  echo
  echo -e -n "\e[36m\$\e[33m $@\e[0m"
  read
  eval $@
}

echo

#open http://localhost:14000/scale-1.0/
#sudo ntpdate 0.fedora.pool.ntp.org

if [ "$SKIP_EAP" == "" ]; then
  run "$base/drive_load.sh &"
  run "$base/drive_load.sh &"

  run gear start localhost/demo-backend-3
  run sudo journalctl --unit ctr-demo-backend-3.service -f --since=-3 -q
fi

gear stop localhost/demo-backend-{1,2,3} > /dev/null &

if [ "$SKIP_MONGO" == "" ]; then
  run cat $base/mongo_Dockerfile

  run gear deploy $base/deploy_mongo_repl_set.json localhost
  run sudo journalctl --unit ctr-replset-db-1 -f --since=-3 -q
  run "sudo switchns --container=replset-db-1 -- /usr/bin/mongo local --eval 'printjson(rs.initiate({_id: \"replica0\", version: 1, members:[{_id: 0, host:\"192.168.1.1:27017\"},{_id: 1, host:\"192.168.1.2:27017\"},{_id: 2, host:\"192.168.1.3:27017\"}]}))'" #; printjson(rs.add(\"192.168.1.2\")); printjson(rs.add(\"192.168.1.3\"))'"
  run "sudo switchns --container=replset-db-1 -- /usr/bin/mongo local --eval 'printjson(rs.status())'"
  run "sudo switchns --container=replset-db-1 -- /usr/bin/mongo local --eval 'printjson(rs.status())'"

  run gear build git://github.com/pmorie/scaling-demo-update jboss/eap openshift/demo-eap-scaling-test
  run gear install openshift/demo-eap-scaling-test localhost/demo-backend-3 -p 8080:0
  run gear start localhost/demo-backend-3
  run sudo journalctl -u ctr-demo-backend-3 -f
fi

gear stop --with=$base/deploy_mongo_repl_set_instances.json > /dev/null &

if [ "$SKIP_ORIGIN" == "" ]; then
  run gear deploy $base/deploy_openshift.json localhost
  run sudo journalctl --unit ctr-openshift-broker-1 -f --since=-3 -q
  run sudo switchns --container="openshift-broker-1" --env="BROKER_SOURCE=1" --env="HOME=/opt/ruby" --env="OPENSHIFT_BROKER_DIR=/opt/ruby/src/broker" -- /bin/bash --login -c "/opt/ruby/src/docker/openshift_init"
  run rhc setup --server http://localhost:6060/broker/rest/api
  run rhc create-app test jbosseap-6.0 --no-git --no-dns
fi
