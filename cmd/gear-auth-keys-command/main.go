package main

import (
	_ "net/http/pprof"
	"os"

	"github.com/openshift/geard/cmd"
)

func main() {
	cmd.ExecuteSshAuthKeysCmd(os.Args...)
}
