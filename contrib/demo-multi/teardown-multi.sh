#!/bin/sh -x

base=$(dirname $0)

gear stop --with=$base/deploy_parks_map_multihost.json
gear delete --with=$base/deploy_parks_map_multihost.json

docker rm parks-db-1-data

sudo du -s -h /var/lib/docker/vfs/dir
