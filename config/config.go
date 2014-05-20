package config

import (
	"os"
	"fmt"
)

func init() {
	var value string

	value = os.Getenv("GEARD_RUN_PATH")
	if 0 != len(value) {
		runPath = value
	}

	value = os.Getenv("GEARD_BASE_PATH")
	if 0 != len(value) {
		basePath = value
	}
}

func ContainerRunPath() string {
	return runPath
}

func SetContainerRunPath(value string) error {
	if 0 == len(value) {
		return fmt.Errorf("SetContainerRunPath: requires a non-empty argument")
	}

	runPath = value
	return nil
}

func ContainerBasePath() string {
	return basePath
}

func SetContainerBasePath(value string) error {
	if 0 == len(value) {
		return fmt.Errorf("SetContainerBasePath: requires a non-empty argument")
	}

	basePath = value
	return nil
}

type DockerConfiguration struct {
	Socket string
}

type DockerFeatures struct {
	EnvironmentFile bool
	ForegroundRun   bool
}

var (
	SystemDockerFeatures = DockerFeatures{}
	basePath             = "/var/lib/containers"
	runPath              = "/var/run/containers"
)
