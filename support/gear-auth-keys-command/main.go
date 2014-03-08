package main

import (
	"github.com/smarterclayton/geard/cmd"
	_ "net/http/pprof"
	"os"
)

func main() {
	cmd.ExecuteSshAuthKeysCmd(os.Args...)
}
