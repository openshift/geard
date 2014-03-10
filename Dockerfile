FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

ENV GOPATH /go
RUN yum install -y golang git hg gcc libselinux-devel && yum clean all
RUN mkdir -p $GOPATH && echo $GOPATH >> ~/.bash_profile

ADD     . /go/src/github.com/smarterclayton/geard
WORKDIR   /go/src/github.com/smarterclayton/geard
RUN \
   go get -tags selinux ./... && \
   go install -tags selinux github.com/smarterclayton/geard/cmd/gear && \
   go install -tags selinux github.com/smarterclayton/geard/support/switchns && \
   go install -tags selinux github.com/smarterclayton/geard/support/gear-auth-keys-command && \
   /bin/cp -f $GOPATH/bin/{gear,switchns,gear-auth-keys-command} /bin/ && \
   rm -rf $GOPATH

# Create an environment for Git execution
ADD contrib/githost/default-hooks/ /home/git/default-hooks
ADD contrib/githost/init           /home/git/init
RUN useradd git --uid 1001 -U && mkdir -p /home/git && chown -R git /home/git

CMD ["/bin/gear", "daemon"]
EXPOSE 8080
VOLUME /var/lib/containers
