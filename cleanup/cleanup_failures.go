package cleanup

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/docker"
	"strings"
	"os"
)

type FailureCleanup struct {
	dockerSocket string
}

func init() {
	dockerURI := os.Getenv("DOCKER_URI")
	if dockerURI == "" {
		dockerURI = "unix:///var/run/docker.sock"
	}

	AddCleaner(&FailureCleanup{dockerSocket: dockerURI})
}

// Remove any geard managed container with a non-zero exit code from runtime
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

	for _, cinfo := range gears {
		container, err := client.GetContainer(cinfo.ID, false)
		if err != nil {
			ctx.LogError.Printf("Unable to retrieve container information for %s: %s", container.Name, err.Error())
			continue
		}

		// Happy container or not under geard control
		if 0 == container.State.ExitCode || !strings.HasPrefix(container.Name, containers.IdentifierPrefix) {
			continue
		}

		if ctx.DryRun {
			ctx.LogInfo.Printf("DryRun: container %s could be removed as it has exit code of %d",
				container.Name, container.State.ExitCode)
			continue
		}

		// Container under geard control and has a non-zero exit code, remove it from registry
		ctx.LogInfo.Printf("Removing container %s has exit code of %d", container.Name, container.State.ExitCode)
		e1 := client.ForceCleanContainer(container.ID)
		if e1 != nil {
			ctx.LogError.Printf("Unable to remove container %s from runtime: %s", container.Name, e1.Error())
		}
	}
}

