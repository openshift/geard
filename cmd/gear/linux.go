// +build linux && !skip_default

package main

import (
	cleancmd "github.com/openshift/geard/cleanup/cmd"
	"github.com/openshift/geard/cmd"
	ctrcmd "github.com/openshift/geard/containers/cmd"
	chttp "github.com/openshift/geard/containers/http"
	cjobs "github.com/openshift/geard/containers/jobs/linux"
	initcmd "github.com/openshift/geard/containers/systemd/init"
	"github.com/openshift/geard/git"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	routercmd "github.com/openshift/geard/router/cmd"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshhttp "github.com/openshift/geard/ssh/http"
	sshjobs "github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/transport"
)

func init() {
	transport.RegisterTransport("http", &http.HttpTransport{})
	defaultTransport.Set("http")

	cmd.AddCommandExtension(registerHttpDaemonCommands, true)

	ctx := ctrcmd.CommandContext{Transport: &defaultTransport.TransportFlag, Insecure: &insecure}
	cmd.AddCommandExtension(ctx.RegisterLocal, true)
	cmd.AddCommandExtension(ctx.RegisterRemote, false)

	cmd.AddCommandExtension(gitcmd.RegisterInitRepo, true)
	a := &gitcmd.CommandContext{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(a.RegisterCreateRepo, false)

	sshcmd.AddPermissionCommand(git.ResourceTypeRepository, &gitcmd.PermissionCommandContext{})

	cmd.AddCommandExtension(sshcmd.RegisterAuthorizedKeys, true)
	b := &sshcmd.CommandContext{Transport: &defaultTransport.TransportFlag}
	cmd.AddCommandExtension(b.RegisterAddKeys, false)

	cmd.AddCommandExtension(cleancmd.RegisterCleanup, true)
	cmd.AddCommandExtension(initcmd.RegisterInit, true)
	cmd.AddCommandExtension(routercmd.RegisterRouter, true)

	jobs.AddJobExtension(cjobs.NewContainerExtension())
	jobs.AddJobExtension(gitjobs.NewGitExtension())
	jobs.AddJobExtension(sshjobs.NewSshExtension())

	http.AddHttpExtension(&chttp.HttpExtension{})
	http.AddHttpExtension(&githttp.HttpExtension{})
	http.AddHttpExtension(&sshhttp.HttpExtension{})
}
