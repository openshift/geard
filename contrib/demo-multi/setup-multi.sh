#!/bin/sh -x

base=$(dirname $0)

set +x

units=$(curl -q http://localhost:43273/containers)
ret=$?
if [ $ret -ne 0 ]; then
  echo "gear daemon not responding, make sure the service is running and retry."
  exit 1
fi

units=$(curl -q http://192.168.205.11:43273/containers)
ret=$?
if [ $ret -ne 0 ]; then
  echo "gear daemon on vm2 not responding, make sure the service is running and retry."
  exit 1
fi

$base/teardown-multi.sh

#gear deploy $base/deploy_parks_map.json localhost localhost 192.168.205.11 192.168.205.11 localhost
#gear stop 192.168.205.11/parks-backend-{2,3}

gear install openshift/centos-mongodb parks-db-1 -p "27017:4003" openshift/centos-mongodb --start
$base/wait_for_url.sh "http://localhost:4003/"
read

gear install parks-map-app parks-backend-1 -p "3000:4002" -n "127.0.0.1:27017:localhost:4003" --start
$base/wait_for_url.sh "http://localhost:4002/"
read

gear install parks-map-app atomic-2:43273/parks-backend-2 -p "3000:4002" -n "127.0.0.1:27017:192.168.205.10:4003" --start
$base/wait_for_url.sh "http://atomic-2:4002"
read

gear install parks-map-app atomic-2:43273/parks-backend-3 -p "3000:4003" -n "127.0.0.1:27017:192.168.205.10:4003" --start
$base/wait_for_url.sh "http://atomic-2:4003"
read

gear install openshift/centos-haproxy-simple-balancer parks-lb-1 -p "8080:14000,1936:15000" -n "192.168.1.1:8080:localhost:4002,192.168.1.2:8080:192.168.205.11:4002,192.168.1.3:8080:192.168.205.11:4003" --start
$base/wait_for_url.sh "http://localhost:14000"
read

$base/db-import.sh
