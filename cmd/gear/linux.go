// +build linux && !skip_default

package main

import (
	cleancmd "github.com/openshift/geard/cleanup/cmd"
	"github.com/openshift/geard/cmd"
	chttp "github.com/openshift/geard/containers/http"
	cjobs "github.com/openshift/geard/containers/jobs"
	initcmd "github.com/openshift/geard/containers/systemd/init"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	routercmd "github.com/openshift/geard/router/cmd"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshhttp "github.com/openshift/geard/ssh/http"
	sshjobs "github.com/openshift/geard/ssh/jobs"
)

func init() {
	cmd.AddCommandExtension(gitcmd.RegisterInitRepo, true)
	a := &gitcmd.Command{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(a.RegisterCreateRepo, false)

	cmd.AddCommandExtension(sshcmd.RegisterAuthorizedKeys, true)
	b := &sshcmd.Command{&defaultTransport.TransportFlag}
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
