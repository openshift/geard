package config

const (
	basePath = "/var/lib/containers"
)

func ContainerBasePath() string {
	return basePath
}

type DockerConfiguration struct {
	Socket string
}

type DockerFeatures struct {
	EnvironmentFile bool
	ForegroundRun   bool
}

var SystemDockerFeatures = DockerFeatures{}
