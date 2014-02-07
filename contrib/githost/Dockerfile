FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

RUN yum install -y git && yum clean all
RUN useradd git

USER git
ADD default-hooks/ /home/git/default-hooks
ADD init /home/git/init
VOLUME /home/git/repo
ENTRYPOINT ["/home/git/init"]