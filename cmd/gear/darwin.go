// +build darwin && !skip_default

package main

import (
	"github.com/openshift/geard/cmd"
	chttp "github.com/openshift/geard/containers/http"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	"github.com/openshift/geard/http"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshhttp "github.com/openshift/geard/ssh/http"
)

func init() {
	a := &gitcmd.Command{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(a.RegisterCreateRepo, false)

	cmd.AddCommandExtension(sshcmd.RegisterAuthorizedKeys, true)
	b := &sshcmd.Command{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(b.RegisterAddKeys, false)

	http.AddHttpExtension(&chttp.HttpExtension{})
	http.AddHttpExtension(&githttp.HttpExtension{})
	http.AddHttpExtension(&sshhttp.HttpExtension{})
}
