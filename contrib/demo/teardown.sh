#!/bin/sh

base=$(dirname $0)

gear stop --with=$base/deploy_mongo_repl_set_instances.json
gear stop --with=$base/deploy_openshift_instances.json
gear stop --with=$base/deploy_eap_cluster_instances.json
gear delete --with=$base/deploy_mongo_repl_set_instances.json
gear delete --with=$base/deploy_openshift_instances.json
gear delete --with=$base/deploy_eap_cluster_instances.json

docker rm replset-db-{1,2,3}-data openshift-broker-1-data openshift-db-1-data demo-backend-{1,2,3}-data demo-db-1-data demo-lb-1-data

sudo du -s -h /var/lib/docker/vfs/dir