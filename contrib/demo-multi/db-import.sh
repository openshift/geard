#!/bin/sh -x

sudo switchns --container=parks-db-1 -- /bin/bash -c "curl https://raw.githubusercontent.com/thesteve0/fluentwebmap/master/parkcoord.json | mongoimport -d fluent -c parkpoints --type json && mongo fluent --eval 'db.parkpoints.ensureIndex( { pos : \"2d\" } );'"
