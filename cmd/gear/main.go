package main

import (
	"github.com/smarterclayton/geard/cmd"
	_ "net/http/pprof"
)

func main() {
	cmd.Execute()
}
