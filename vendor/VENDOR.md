Go get does not support versioning of packages. So, the recommended way of using external libraries is to use the "vendoring" method,
where this library code is copied in your repository at a specific revision.

As a general rule; libraries created for this project specifically should go into pkg/ directory while external libraries or forked
libraries should go into the vendor/ directory.

Getting started
===============

For geard, a script is provided which will help with vendoring packages.
  1. Ensure that the package you wish to vendor is not in your gopath
  2. Run ```./contrib/vendor -l``` to get a list of packages which may need to be vendored
  3. Run ```./contrib/vendor -v <Package URL>``` to vendor the package
     Eg: ```./contrib/vendor -v github.com/fsouza/go-dockerclient```

Updating vendored packages
==========================

Vendoring sets up a subtree. See [subtree docs](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt) for more detail on merging/splitting subtrees.

Note: Since you cannot push the configuration for upstreams, you will need to recreate upstreams. Example upstream in your .git/config will looks like:

    [remote "vndr_github_com_fsouza_go-dockerclient"]
            url = http://github.com/fsouza/go-dockerclient
            fetch = +refs/heads/*:refs/remotes/vndr_github_com_fsouza_go-dockerclient/*
    [branch "subtree_github_com_fsouza_go-dockerclient"]
            remote = vndr_github_com_fsouza_go-dockerclient
            merge = refs/heads/master

Corresponding commands would be:

    git remote add vndr_github_com_fsouza_go-dockerclient http://github.com/fsouza/go-dockerclient
    git branch -u vndr_github_com_fsouza_go-dockerclient/master subtree_github_com_fsouza_go-dockerclient
