package init

import (
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"os"
)

func generateContainerIsolation(image docker.Image, opt docker.CreateContainerOptions) (docker.CreateContainerOptions, error) {
	data := isolateInitScript{
		image.Config.User == "",
		user,
		u.Uid,
		u.Gid,
		strings.Join(image.Config.Cmd, " "),
		len(volumes) > 0,
		strings.Join(volumes, " "),
	}

	file, _, err := utils.OpenFileExclusive(path.Join(id.RunPathFor(), "container-init.sh"), 0700)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := isolateInitTemplate.Execute(file, data); erre != nil {
		fmt.Printf("container init pre-start: Unable to output template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	file, _, err = utils.OpenFileExclusive(path.Join(id.RunPathFor(), "container-cmd.sh"), 0705)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open cmd script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := isolateCmdTemplate.Execute(file, data); erre != nil {
		fmt.Printf("container init pre-start: Unable to output cmd template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}
}

type isolateInitScript struct {
	CreateUser    bool
	ContainerUser string
	Uid           string
	Gid           string
	Command       string
	HasVolumes    bool
	Volumes       string
}

var isolateInitTemplate = template.Must(template.New("container-init.sh").Parse(`#!/bin/sh
{{ if .CreateUser }}
if command -v useradd >/dev/null; then
  useradd -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}
else
  adduser -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}
fi
{{ else }}
old_id=$(id -u {{.ContainerUser}})
old_gid=$(id -g {{.ContainerUser}})
/usr/sbin/usermod --uid {{.Uid}} {{.ContainerUser}}
/usr/sbin/groupmod --gid {{.Gid}} {{.ContainerUser}}
for i in $(find / -uid ${old_id}); do PATH=/bin:/sbin:/usr/bin:/usr/sbin chown -R {{.Uid}} $i; done
for i in $(find / -gid ${old_gid}); do PATH=/bin:/sbin:/usr/bin:/usr/sbin chgrp -R {{.Gid}} $i; done
{{ end }}
{{ if .HasVolumes }}
chown -R {{.Uid}}:{{.Gid}} {{.Volumes}}
{{ end }}
exec su {{.ContainerUser}} -s /.container.init/container-cmd.sh
`))

var isolateCmdTemplate = template.Must(template.New("container-cmd.sh").Parse(`#!/bin/sh
exec {{.Command}}
`))
