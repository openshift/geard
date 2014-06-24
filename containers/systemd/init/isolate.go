package init

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"text/template"
)

type Isolator struct {
	RunPath string
	User    *user.User

	initFile *os.File
	cmdFile  *os.File
}

func (i *Isolator) Update(image docker.Image, opt docker.CreateContainerOptions) (docker.CreateContainerOptions, error) {
	volumes := []string{}
	for volume := range image.Config.Volumes {
		volumes = append(volumes, volume)
	}
	cmd := image.Config.Entrypoint
	if len(cmd) == 0 {
		cmd = image.Config.Cmd
	}
	user := opt.Config.User
	if user == "" {
		user = image.Config.User
	}

	data := isolateInitScript{
		user == "",
		user,
		i.User.Uid,
		i.User.Gid,
		strings.Join(cmd, " "),
		len(volumes) > 0,
		strings.Join(volumes, " "),
	}

	initFile, err := writeTemplateToTempFile(isolateInitTemplate, data, "", "container-init-sh")
	if err != nil {
		return opt, fmt.Errorf("unable to write init script: %v", err)
	}

	cmdFile, err := writeTemplateToTempFile(isolateCmdTemplate, data, "", "container-cmd-sh")
	if err != nil {
		os.Remove(initFile.Name())
		return opt, fmt.Errorf("unable to write cmd script: %v", err)
	}

	i.initFile = initFile
	i.cmdFile = cmdFile

	opt.Config.User = "root"
	opt.Config.Entrypoint = []string{"/.container.init/container-init.sh"}

	return opt, nil
}

func (i *Isolator) Alter(container docker.Container, client *docker.Client) error {
	log.Printf("container: %+v", container)
	info, _ := client.Info()
	driver := info.Map()["Driver"]
	log.Printf("info: %+v", info)
	return nil
}

func (i *Isolator) Close() error {
	os.Remove(i.cmdFile.Name())
	os.Remove(i.initFile.Name())
	return nil
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

func writeTemplateToTempFile(t *template.Template, data interface{}, path, prefix string) (*os.File, error) {
	file, err := ioutil.TempFile(path, prefix)
	if err != nil {
		return nil, err
	}
	if err := t.Execute(file, data); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	if err := file.Close(); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}
