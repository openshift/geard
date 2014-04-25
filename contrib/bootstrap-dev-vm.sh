#!/bin/bash
echo "Installing dependencies and setting up vm for geard development"
yum update -y
yum install -y docker-io golang git hg bzr libselinux-devel
yum install -y vim tig glibc-static btrfs-progs-devel device-mapper-devel sqlite-devel libnetfilter_queue-devel gcc gcc-c++
usermod -a -G docker vagrant
systemctl enable docker.service
systemctl start docker
systemctl status docker

mkdir -p /vagrant/{src/github.com/openshift/geard,pkg,bin}
GEARD_PATH=/vagrant/src/github.com/openshift/geard
chown -R vagrant:vagrant /vagrant

# Modify SSHD config to use gear-auth-keys-command to support git clone from repo
echo 'AuthorizedKeysCommand /usr/sbin/gear-auth-keys-command' >> /etc/ssh/sshd_config
echo 'AuthorizedKeysCommandUser nobody' >> /etc/ssh/sshd_config

echo 'export GOPATH=/vagrant' >> ~vagrant/.bash_profile
echo 'export PATH=$GOPATH/bin:$PATH' >> ~vagrant/.bash_profile
echo "cd $GEARD_PATH" >> ~vagrant/.bashrc
echo "bind '\"\e[A\":history-search-backward'" >> ~vagrant/.bashrc
echo "bind '\"\e[B\":history-search-forward'" >> ~vagrant/.bashrc

echo "Docker access will be enabled when you 'vagrant ssh' in."
echo "Run 'contrib/build' to build your source."
