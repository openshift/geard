package main

import (
	"fmt"
	"github.com/docopt/docopt.go"
	"github.com/fsouza/go-dockerclient"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/support/gear-setup/util"
	d "github.com/smarterclayton/geard/support/gear-setup/util/docker"
	"os"
	"os/user"
	"strconv"
)

func main() {
	usage := `Geard Utilities

Usage:
	gear-setup pre-start (<gear name> | --username=<user login>)
	gear-setup post-start (<gear name> | --username=<user login>)
	gear-setup gen-auth-key
`
	var arguments map[string]interface{}
	var err error
	if arguments, err = docopt.Parse(usage, nil, true, "GearD Utilities", false); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var id gears.Identifier
	var u *user.User

	switch {
	case arguments["gen-auth-key"] == true:
		uid := os.Getuid()
		if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
			fmt.Println(err)
			os.Exit(5)
		}
		if id, err = gears.NewIdentifierFromUser(u); err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
	case arguments["<gear name>"] != nil:
		if id, err = gears.NewIdentifier(arguments["<gear name>"].(string)); err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
		if u, err = user.Lookup(id.LoginFor()); err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
	case arguments["--username"] != nil:
		if u, err = user.Lookup(arguments["--username"].(string)); err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		if id, err = gears.NewIdentifierFromUser(u); err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
	}

	switch {
	case arguments["gen-auth-key"] == true:
		var container *docker.Container
		if _, container, err = d.GetContainer(id.LoginFor(), false); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		if err := util.GenerateAuthorizedKeys(id, u, container, false); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	case arguments["post-start"] == true:
		var container *docker.Container
		if _, container, err = d.GetContainer(id.LoginFor(), true); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		if err := util.GenerateAuthorizedKeys(id, u, container, true); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		
		//todo: IPTables bsed link setup
	}
}
