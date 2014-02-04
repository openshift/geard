FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

ENV GOPATH /go/src
RUN yum install -y golang git && yum clean all
RUN mkdir -p $GOPATH && echo $GOPATH >> ~/.bash_profile

ADD . /geard
WORKDIR /geard
RUN go get -d
RUN go build -o geard.local

CMD /geard/geard.local

EXPOSE 8080
VOLUME /var/lib/gears