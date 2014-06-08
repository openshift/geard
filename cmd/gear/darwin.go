// +build darwin && !skip_default

package main

import (
	"github.com/openshift/geard/cmd"
	ctrcmd "github.com/openshift/geard/containers/cmd"
	chttp "github.com/openshift/geard/containers/http"
	"github.com/openshift/geard/git"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/http/client"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshhttp "github.com/openshift/geard/ssh/http"
	"github.com/openshift/geard/transport"
)

func init() {
	transport.RegisterTransport("http", &client.HttpTransport{})
	defaultTransport.Set("http")

	ctx := ctrcmd.CommandContext{Transport: &defaultTransport.TransportFlag, Insecure: &insecure}
	cmd.AddCommandExtension(ctx.RegisterLocal, true)
	cmd.AddCommandExtension(ctx.RegisterRemote, false)

	a := &gitcmd.CommandContext{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(a.RegisterCreateRepo, false)

	sshcmd.AddPermissionCommand(git.ResourceTypeRepository, &gitcmd.PermissionCommandContext{})

	cmd.AddCommandExtension(sshcmd.RegisterAuthorizedKeys, true)
	b := &sshcmd.CommandContext{Transport: &defaultTransport.TransportFlag}
	cmd.AddCommandExtension(b.RegisterAddKeys, false)

	http.AddHttpExtension(&chttp.HttpExtension{})
	http.AddHttpExtension(&githttp.HttpExtension{})
	http.AddHttpExtension(&sshhttp.HttpExtension{})
}
