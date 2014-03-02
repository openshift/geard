package config

const basePath = "/var/lib/gears"

func GearBasePath() string {
	return basePath
}

type DockerConfiguration struct {
	Socket string
}
