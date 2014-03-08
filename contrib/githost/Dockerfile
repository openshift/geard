FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

RUN yum install -y git && yum clean all && mkdir -p /git
ADD default-hooks/ /git/default-hooks
ADD init /git/init
ADD init-repo /git/init-repo
VOLUME /var/lib/containers/git
VOLUME /host_etc
RUN rm -f /etc/passwd && ln -sf /host_etc/passwd /etc/passwd
ENTRYPOINT ["/git/init"]
