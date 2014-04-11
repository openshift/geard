#!/bin/bash -e

pushd $HOME

sudo yum install -y wget java-1.8.0-openjdk-devel.x86_64 gcc gcc-c++ python-devel cyrus-sasl-devel.x86_64 libcurl-devel patch protobuf-compiler mlocate protobuf-devel
sudo updatedb

if [ ! -f mesos-0.17.0.tar.gz ]; then
  wget http://mirror.olnevhost.net/pub/apache/mesos/0.17.0/mesos-0.17.0.tar.gz
fi

if [ ! -d "${HOME}/mesos-0.17.0" ]; then
  tar zxf mesos-0.17.0.tar.gz
  mkdir -p $HOME/mesos-0.17.0/build
fi

if [ ! -f "/usr/sbin/mesos-master" ]; then
  pushd ${HOME}/mesos-0.17.0/build
    ../configure --prefix=/usr
    make
    sudo make install
  popd
fi

if [ ! -d $GOPATH/src/github.com/kraman/mesos-go ]; then
  go get -d github.com/kraman/mesos-go || true
  pushd $GOPATH/src/github.com/kraman/mesos-go
    make
  popd
fi
