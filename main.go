package main

import (
	"fmt"
	_ "net/http/pprof"
)

var msg = `To build binaries:

go get github.com/smarterclayton/geard/cmd/gear
go get github.com/smarterclayton/geard/cmd/switchns
go get github.com/smarterclayton/geard/cmd/gear-auth-keys-command
`

func main() {
	fmt.Println()
}
