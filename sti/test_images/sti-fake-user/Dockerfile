FROM sti_test/sti-fake

RUN mkdir -p /sti-fake && \
    addgroup -g 433 fakeuser && \
    adduser -D -u 431 -h /sti-fake -s /sbin/nologin fakeuser
    chown -R fakeuser:fakeuser /sti-fake

USER fakeuser
ENV STI_SCRIPTS_URL https://raw.githubusercontent.com/openshift/geard/master/sti/test_images/sti-fake/.sti/bin
                
