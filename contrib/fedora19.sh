#!/bin/bash
sudo yum install -y docker-io golang git hg bzr
sudo usermod -a -G docker vagrant
sudo systemctl enable docker.service
sudo systemctl start docker
sudo systemctl status docker

mkdir -p ~/go/src
echo 'export GOPATH=~/go/src' >> ~/.bash_profile

echo vi /usr/lib/systemd/service/docker.service
echo Set run command to docker -d -H :2223 -H unix:///var/run/docker.sock
echo
echo You\'ll need to log out and in again to enable docker access from your 
echo group and enable go compilation.
