#!/bin/sh
dir=$(dirname $0)
pushd $dir/sti-fake
  docker build -t sti_test/sti-fake .
popd

pushd $dir/sti-fake-user
  docker build -t sti_test/sti-fake-user .
popd

pushd $dir/sti-fake-broken
  docker build -t sti_test/sti-fake-broken .
popd
