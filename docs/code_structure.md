geard code structure
====================

Geard is a large project.  This doc will present the basic layout of the geard source and key 
architectural patterns required to understand it.  Geard is written in Go; we recommend the 
following resources for learning the language:

*   [Golang tour](http://tour.golang.org/#1)
*   [Effective Go](http://golang.org/doc/effective_go.html)
*   [Go Language Specification](http://golang.org/ref/spec)

key patterns
------------

#### Package vendoring

Geard has many dependencies on other projects and uses vendoring to control which versions of these
dependencies are used when the project is built.  The vendored sources are in the `vendor` package.
The `contrib/build` script places the sources in this package in front of the system `GOPATH`,
ensuring that the right versions are loaded regardless of the versions of our dependencies that may
be installed in your local system.  We use the `contrib/vendor` script to maintain our vendored
dependencies.

#### The gear binary and extensions

The `gear` binary is designed to implement a minimal set of core functionality (installing,
starting, stopping containers, etc) and provide a registration point for arbitrary extensions. Many
packages in the geard source supply extensions to this binary via a `cmd` subpackage.  Some
examples from the [package map](#package-map) are:

    geard/
      git/                        # git repositories and their concerns:
        cmd/                      #   gear 'create-repo' extension
      ...
      idler/                      # idler daemon and its concerns:
        cmd/                      #   gear 'idler-daemon' extension
      ...
      router/                     # a test router implementation
        cmd/                      #   gear 'test-router' extension

The extension point is described in [`cmd/extension.go`]():

    // Register flags and commands underneath a parent Command-
    type CommandRegistration func(parent *cobra.Command)

    // ...
    
    // Register an extension to this server during init() or startup
    func AddCommandExtension(ext CommandRegistration, local bool) {

The convention for registering extensions is to create a `cmd` package with `CommandRegistration`
functions.  The gear binary implementation for each supported platform handles calling 
`AddCommandExtension` in its package `init` method.

#### Request handling in the daemon

The geard daemon - started with `gear daemon` presents a rest API for working with containers.
Internally, requests to this API are fulfilled using an implementation of the 
[reactor pattern](http://en.wikipedia.org/wiki/Reactor_pattern).  The reactor is called a 
'dispatcher' in this project; the source is in the `dispatcher` package.  The reactor's units of
work are called 'jobs' in this project; they are described in the `jobs` package.


binaries built
--------------

    gear                          # Core binary + extensions
    sti                           # Source-to-images (STI) binary
    switchns                      # Switchns - change namespace into a container
    gear-auth-keys-command        # SSH AuthorizedKeysCommand plugin for gears

package map
-----------

Check out the [godoc](http://godoc.org/github.com/openshift/geard) for a view of what each package provides.