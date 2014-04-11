FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

ENV GOPATH /go
RUN yum install -y golang git hg bzr libselinux-devel glibc-static btrfs-progs-devel device-mapper-devel sqlite-devel libnetfilter_queue-devel gcc gcc-c++ && yum clean all
RUN mkdir -p $GOPATH && echo $GOPATH >> ~/.bash_profile

ADD     . /go/src/github.com/openshift/geard
WORKDIR   /go/src/github.com/openshift/geard
RUN \
   ./contrib/build -s -n && \
   ./contrib/test && \
   /bin/cp -f $GOPATH/bin/gear-auth-keys-command /usr/sbin/ && \
   /bin/cp -f $GOPATH/bin/switchns /usr/bin && \
   /bin/cp -f $GOPATH/bin/gear /usr/bin && \
   rm -rf $GOPATH

CMD ["/bin/gear", "daemon"]
EXPOSE 43273
VOLUME /var/lib/containers
