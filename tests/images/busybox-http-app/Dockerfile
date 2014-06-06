FROM  openshift/busybox-http

ADD index.html  /usr/mock/source/index.html

CMD ["/usr/sbin/httpd", "-f", "-h", "/usr/mock/source", "-p", "8080"]
