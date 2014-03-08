FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

ENV GOPATH /go
RUN yum install -y golang git hg gcc libselinux-devel && yum clean all
RUN mkdir -p $GOPATH && echo $GOPATH >> ~/.bash_profile

ADD     . /go/src/github.com/smarterclayton/geard
WORKDIR   /go/src/github.com/smarterclayton/geard
RUN \
   go get -tags selinux ./... && \
   go build -tags selinux -o gear . && \
   /bin/cp ./gear /bin/gear && \
   go install -tags selinux ./support/switchns && \
   mkdir -p /opt/geard/bin && \
   /bin/cp -f $GOPATH/bin/switchns /opt/geard/bin && \
   rm -rf $GOPATH

# Create an environment for Git execution
ADD contrib/githost/default-hooks/ /home/git/default-hooks
ADD contrib/githost/init           /home/git/init
RUN useradd git --uid 1001 -U && mkdir -p /home/git && chown -R git /home/git

CMD ["/bin/gear", "daemon"]
EXPOSE 8080
VOLUME /var/lib/containers
