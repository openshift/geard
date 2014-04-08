package main

import (
	_ "net/http/pprof"
	"os"
	"os/user"

	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/ssh"

	//Extentions
	_ "github.com/openshift/geard/git/cmd"
)

func main() {
	if len(os.Args) != 2 {
		Fail(1, "Valid arguments: <login name>\n")
	}

	u, err := user.Lookup(os.Args[1])
	if err != nil {
		Fail(2, "Unable to lookup user")
	}

	if err := ssh.GenerateAuthorizedKeysFor(u, false, true); err != nil {
		Fail(1, "Unable to generate authorized_keys file: %s", err.Error())
	}
}
