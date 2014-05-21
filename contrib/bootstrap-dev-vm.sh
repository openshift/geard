#!/bin/bash
echo "Installing dependencies and setting up vm for geard development"
yum update -y
yum install -y docker-io golang git hg bzr libselinux-devel vim tig glibc-static btrfs-progs-devel device-mapper-devel sqlite-devel libnetfilter_queue-devel gcc gcc-c++
usermod -a -G docker vagrant
systemctl enable docker.service
systemctl start docker
systemctl status docker

mkdir -p /vagrant/{src/github.com/openshift/geard,pkg,bin}
GEARD_PATH=/vagrant/src/github.com/openshift/geard
chown -R vagrant:vagrant /vagrant

# Modify SSHD config to use gear-auth-keys-command to support git clone from repo
if [[ $(cat /etc/ssh/sshd_config | grep /gear-auth-keys-command) = "" ]]; then
  echo 'AuthorizedKeysCommand /usr/sbin/gear-auth-keys-command' >> /etc/ssh/sshd_config
  echo 'AuthorizedKeysCommandUser nobody' >> /etc/ssh/sshd_config
else
  echo "AuthorizedKeysCommand already configured"
fi

# SET GOPATH, GEARD_PATH
cat > /etc/profile.d/geard.sh <<DELIM
export GOPATH=/vagrant
export PATH=$GOPATH/bin:$PATH
DELIM

# SET VAGRANT USER PATH VARIABLES
if [[ $(cat ~vagrant/.bash_profile | grep GOPATH) = "" ]]; then
  echo "cd $GEARD_PATH" >> ~vagrant/.bashrc
  echo "bind '\"\e[A\":history-search-backward'" >> ~vagrant/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> ~vagrant/.bashrc
else
  echo "vagrant user path variables already configured"
fi

# SET ROOT USER PATH VARIABLES
if [[ $(cat /root/.bash_profile | grep GOPATH) = "" ]]; then
  echo "cd $GEARD_PATH" >> /root/.bashrc
  echo "bind '\"\e[A\":history-search-backward'" >> /root/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> /root/.bashrc
else
  echo "root user path variables already configured"
fi

echo "Performing initial geard build..."
su --login --shell="/bin/bash" --session-command "cd $GEARD_PATH && contrib/build" vagrant

echo "Docker access will be enabled when you 'vagrant ssh' in."
echo "Run 'contrib/build' to build your source."
