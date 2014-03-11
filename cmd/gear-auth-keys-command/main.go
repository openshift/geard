package main

import (
	_ "net/http/pprof"
	"os"

	"github.com/smarterclayton/geard/cmd"
)

func main() {
	cmd.ExecuteSshAuthKeysCmd(os.Args...)
}
