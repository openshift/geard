FROM mrunalp/busybox

RUN mkdir -p /usr/mock/source

ADD index.html /usr/mock/source/index.html

RUN ln -sf /usr/sbin/iptables /sbin/iptables

ENV STI_SCRIPTS_URL https://raw.githubusercontent.com/mfojtik/busybox-http/master/.sti/bin

EXPOSE 8080

CMD /bin/sh -c "echo 'This is STI build image.'"
