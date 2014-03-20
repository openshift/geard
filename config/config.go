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
