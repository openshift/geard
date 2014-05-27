package config

import (
	"fmt"
	"os"
)

func init() {
	SetContainerBasePath(os.Getenv("GEARD_BASE_PATH"))
	SetContainerRunPath(os.Getenv("GEARD_RUN_PATH"))
}

func ContainerRunPath() string {
	return runPath
}

func SetContainerRunPath(value string) error {
	if "" == value {
		return fmt.Errorf("SetContainerRunPath: requires a non-empty argument")
	}

	runPath = value
	return nil
}

func ContainerBasePath() string {
	return basePath
}

func SetContainerBasePath(value string) error {
	if "" == value {
		return fmt.Errorf("SetContainerBasePath: requires a non-empty argument")
	}

	basePath = value
	return nil
}

func SystemdBasePath() string {
	return systemdBasePath
}

func SetSystemdBasePath(value string) error {
	if "" == value {
		return fmt.Errorf("SetSystemdBasePath: requires a non-empty argument")
	}

	systemdBasePath = value
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
	systemdBasePath      = "/etc/systemd/system"
)
