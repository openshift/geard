FROM fedora
MAINTAINER Clayton Coleman <ccoleman@redhat.com>

RUN yum install -y git && yum clean all
ADD default-hooks/ /home/git/default-hooks
ADD init /home/git/init
RUN useradd git --uid 1001 -U && mkdir -p /home/git && chown -R git /home/git
USER git
VOLUME /home/git/repo
ENTRYPOINT ["/home/git/init"]