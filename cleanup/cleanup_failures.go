package cleanup

import (
	"github.com/openshift/geard/docker"
	"os"
	"time"
	"strings"
)

type FailureCleanup struct {
	dockerSocket string
	retentionAge string
}

func init() {
	dockerURI := os.Getenv("DOCKER_URI")
	if dockerURI == "" {
		dockerURI = "unix:///var/run/docker.sock"
	}

	AddCleaner(&FailureCleanup{dockerSocket: dockerURI, retentionAge: "72h"})
}

// Remove any geard managed container with certain criteria from runtime
// * exit code non-zero and not running
// * older than retentionAge
// * name does not have -data suffix
func (r *FailureCleanup) Clean(ctx *CleanerContext) {
	ctx.LogInfo.Println("--- \"FAILED CONTAINERS\" CLEANUP ---")

	client, err := docker.GetConnection(r.dockerSocket)
	if err != nil {
		ctx.LogError.Printf("Unable connect to docker: %s. Is daemon running?", r.dockerSocket)
		return
	}

	gears, err := client.ListContainers()
	if err != nil {
		ctx.LogError.Printf("Unable to find any containers: %s", err.Error())
		return
	}

	retentionAge, err := time.ParseDuration(r.retentionAge)
	for _, cinfo := range gears {
		container, err := client.GetContainer(cinfo.ID, false)
		if err != nil {
			ctx.LogError.Printf("Unable to retrieve container information for %s: %s", container.Name, err.Error())
			continue
		}

		// Happy container or not...
		if 0 == container.State.ExitCode ||
				container.State.Running ||
				strings.HasSuffix(container.Name, "-data") ||
				time.Since(container.State.FinishedAt) < retentionAge {
			continue
		}

		// Container under geard control and has a non-zero exit code, remove it from runtime
		ctx.LogInfo.Printf("Removing container %s has exit code of %d", container.Name, container.State.ExitCode)

		if ctx.DryRun {
			continue
		}

		e1 := client.ForceCleanContainer(container.ID)
		if e1 != nil {
			ctx.LogError.Printf("Unable to remove container %s from runtime: %s", container.Name, e1.Error())
		}
	}
}

