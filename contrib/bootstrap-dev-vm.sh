#!/bin/bash
echo "Installing dependencies and setting up vm for geard development"

yum install -y docker-io golang git hg bzr libselinux-devel
usermod -a -G docker vagrant
systemctl enable docker.service
systemctl start docker
systemctl status docker

# Setup local fork to play nice with 'go'
mkdir -p ~vagrant/go/src/github.com/smarterclayton
chown -R vagrant:vagrant ~vagrant/go
ln -s /vagrant ~vagrant/go/src/github.com/smarterclayton/geard
chown -h vagrant:vagrant ~vagrant/go/src/github.com/smarterclayton/geard

# Install / enable systemd unit
cp -f /vagrant/contrib/geard.service /usr/lib/systemd/system/geard.service
systemctl enable /usr/lib/systemd/system/geard.service

echo 'export GOPATH=~/go' >> ~vagrant/.bash_profile
echo 'export PATH=$GOPATH:$PATH' >> ~vagrant/.bash_profile
echo 'cd /vagrant' >> ~vagrant/.bashrc

echo "Docker access will be enabled when you 'vagrant ssh' in"
