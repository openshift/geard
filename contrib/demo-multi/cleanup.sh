systemctl stop geard
docker stop $(docker ps -a -q)
docker rm $(docker ps -a -q)
rm -fr /var/lib/containers
systemctl start geard
