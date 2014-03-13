#!/bin/bash
echo "Installing dependencies and setting up vm for geard development"
yum update -y
yum install -y docker-io golang git hg bzr libselinux-devel
yum install -y vim tig glibc-static btrfs-progs-devel device-mapper-devel sqlite-devel
usermod -a -G docker vagrant
systemctl enable docker.service
systemctl start docker
systemctl status docker

GEARD_PATH=/vagrant/src/github.com/smarterclayton/geard

# Install / enable systemd unit
cp -f $GEARD_PATH/contrib/geard.service /usr/lib/systemd/system/geard.service
docker pull ccoleman/geard
docker pull kraman/githost
docker pull pmorie/sti-html-app
systemctl enable /usr/lib/systemd/system/geard.service
systemctl start /usr/lib/systemd/system/geard.service

echo 'export GOPATH=/vagrant' >> ~vagrant/.bash_profile
echo 'export PATH=$GOPATH/bin:$PATH' >> ~vagrant/.bash_profile
echo "cd $GEARD_PATH" >> ~vagrant/.bashrc
echo "bind '\"\e[A\":history-search-backward'" >> ~vagrant/.bashrc
echo "bind '\"\e[B\":history-search-forward'" >> ~vagrant/.bashrc

echo "stop and & disable geard.service if are compiling it locally"
echo "systemctl stop /usr/lib/systemd/system/geard.service"
echo "systemctl disable /usr/lib/systemd/system/geard.service"

echo "Docker access will be enabled when you 'vagrant ssh' in."
echo "Run 'contrib/build' to build your source."
