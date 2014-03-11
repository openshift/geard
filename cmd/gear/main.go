package main

import (
	_ "net/http/pprof"

	"github.com/smarterclayton/geard/cmd"
)

func main() {
	cmd.Execute()
}
