// +build !no_extensions

package main

import (
	// Extensions
	_ "github.com/openshift/geard/git/cmd"
	_ "github.com/openshift/geard/idler/cmd"
	_ "github.com/openshift/geard/router/cmd"
	_ "github.com/openshift/geard/ssh/cmd"

	// Debug only
	_ "net/http/pprof"
)
