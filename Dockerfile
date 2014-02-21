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
   rm -rf $GOPATH

   #go get -tags selinux "github.com/kraman/geard-switchns" && \
   #go get -tags selinux "github.com/kraman/geard-util" && \
   #mkdir /opt/geard/bin && /bin/cp $GOPATH/bin/geard-switchns $GOPATH/bin/geard-util /bin && \

# Create an environment for Git execution
ADD contrib/githost/default-hooks/ /home/git/default-hooks
ADD contrib/githost/init           /home/git/init
RUN useradd git --uid 1001 -U && mkdir -p /home/git && chown -R git /home/git

CMD ["/bin/gear", "-d"]
EXPOSE 8080
VOLUME /var/lib/gears