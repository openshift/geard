geard code structure
====================

Geard is a large project.  This is a rough map of each directory and its contents:

    geard/
      cleanup/                    # clean up tools - delete old unit files
          cmd/                    #   gear 'clean' extension
      cmd/                        # various binaries built from geard source:
        gear/                     #   gear cli binary
        sti/                      #   source-to-images (sti) binary
        switchns/                 #   switchns binary
      config/                     # configuration items for geard (container path, etc)
      containers/                 # containers and their concerns:
        http/                     #   http requests and handlers (glue code)
        jobs/                     #   job implementations for core API
        systemd/                  #   gears and systemd
          init/                   #     gear init extension - setup the systemd fixtures
      contrib/                    # contributions - various scripts and fixtures
      deployment/                 # gear deployment/orchestration code
        fixtures/                 #   fixtures for deployment tests
      dispatcher/                 # reactor pattern implementation - processes jobs
      docker/                     # methods for working with docker
      docs/                       # documentation
      encrypted/                  # encryption tokens and handlers for encrypted jobs
        fixtures/                 #   fixtures for encryption tests
      git/                        # git repositories and their concerns:
        cmd/                      #   gear create-repo extension
        http/                     #   http requests and handlers (glue code)
        jobs/                     #   job implementations
      http/                       # http transport for jobs
      idler/                      # idler daemon and its concerns:
        cmd/                      #   gear idler extension
        config/                   #   idler configuration
        iptables/                 #   change iptables rules for container idle/unidle
      jobs/                       # job framework (reactor units of work)
      pkg/                        # other projects in our group that geard depends on
      port/                       # port allocation and reservation
      router/                     # a test router implementation
        cmd/                      #   gear test-router extension
      selinux/                    # gears and selinux policies
        library/                  #   golang libselinux wrapper
      ssh/                        # container ssh
        cmd/                      #   ssh binaries 
          gear-auth-keys-command/ #     gear-auth-keys-command binary
        http/                     #   http requests and handlers (glue code)
        jobs/                     #   job implementations
      sti/                        # docker source to images (sti) library:
        contrib/                  #   contrib scripts for old project (will be removed soon)
        test_images/              #   dockerfiles for sti test fixture images
      systemd/                    # utilities for calling systemd and using the journal
      tests/                      # integration tests for geard
      utils/                      # basic internal utilities for geard
      vendor/                     # external dependencies
