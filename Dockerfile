FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

ENV GOPATH /go/src
RUN yum install -y golang git hg && yum clean all
RUN mkdir -p $GOPATH && echo $GOPATH >> ~/.bash_profile

ADD . /geard
WORKDIR /geard
RUN go get -d
RUN go build -o geard.local geard/main.go

CMD /geard/geard.local
EXPOSE 8080
VOLUME /var/lib/gears

# Create an environment for Git execution
ADD contrib/githost/default-hooks/ /home/git/default-hooks
ADD contrib/githost/init           /home/git/init
RUN useradd git --uid 1001 -U && mkdir -p /home/git && chown -R git /home/git