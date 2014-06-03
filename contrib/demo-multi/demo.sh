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

run gear start localhost/parks-backend-2
run sudo journalctl --unit ctr-parks-backend-2.service -f --since=-3 -q

run gear build https://github.com/smarterclayton/fluentwebmap.git nodejs-centos parks-map-app-new
run gear install parks-map-app-new localhost/parks-backend-3
run gear start localhost/parks-backend-3
run sudo journalctl --unit ctr-parks-backend-3.service -f --since=-3 -q
