package config

import (
	"os"
)

const (
	basePath = "/var/lib/containers"
	runPath  = "/var/run/containers"
)

func ContainerRunPath() string {
	path := os.Getenv("GEARD_RUN_PATH")
	if 0 == len(path) {
		return runPath
	}
	return path
}

func ContainerBasePath() string {
	path := os.Getenv("GEARD_BASE_PATH")
	if 0 == len(path) {
		return basePath
	}
	return path
}

type DockerConfiguration struct {
	Socket string
}

type DockerFeatures struct {
	EnvironmentFile bool
	ForegroundRun   bool
}

var SystemDockerFeatures = DockerFeatures{}
