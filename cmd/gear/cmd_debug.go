// +build !skip_debug

package main

import (
	// Debug only
	_ "net/http/pprof"
)
