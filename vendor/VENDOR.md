Go get does not support versioning of packages. So, the recommended way of using external libraries is to use the "vendoring" method,
where this library code is copied in your repository at a specific revision.

As a general rule; libraries created for this project specifically should go into pkg/ directory while external libraries or forked
libraries should go into the vendor/ directory.

For geard, a script is provided which will help with vendoring packages.
  1. Ensure that the package you wish to vendor is not in your gopath
  2. Run "./contrib/vendor -l" to get a list of packages which may need to be vendored
  2. Run "./contrib/vendor -v <Package URL>" to vendor the package

