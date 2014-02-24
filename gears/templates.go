package gears

import (
	"text/template"
)

type ContainerUnit struct {
	Gear            Identifier
	Image           string
	PortSpec        string
	Slice           string
	User            string
	ReqId           string
	GearBasePath    string
	HomeDir         string
	EnvironmentPath string
	Prestart        bool
	Poststart       bool
}

var ContainerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Gear container {{.Gear}}

[Service]
Type=simple
Slice={{.Slice}}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
{{ if .Prestart }}
ExecStartPre={{.GearBasePath}}/bin/gear init --pre "{{.Gear}}" "{{.Image}}"
ExecStart=/usr/bin/docker run -name "gear-{{.Gear}}" -volumes-from "gear-{{.Gear}}" -v {{.HomeDir}}/gear-init.sh:/.gear.init:ro -u root -a stdout -a stderr {{.PortSpec}} -rm "{{.Image}}" /.gear.init
{{else}}
ExecStart=/usr/bin/docker run -name "gear-{{.Gear}}" -volumes-from "gear-{{.Gear}}" -a stdout -a stderr {{.PortSpec}} -rm "{{.Image}}"
{{ end }}
{{ if .Poststart }}
ExecStartPost={{.GearBasePath}}/bin/gear init --post "{{.Gear}}" "{{.Image}}"
{{ end }}

[Install]
WantedBy=gear.target

# Gear information
X-GearId={{.Gear}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
`))

type GearInitScript struct {
	CreateUser    bool
	ContainerUser string
	Uid           string
	Gid           string
	Command       string
	HasVolumes    bool
	Volumes       string
}

var GearInitTemplate = template.Must(template.New("gear-init.sh").Parse("#!/bin/bash\n" +
	"{{ if .CreateUser }}\n" +
	"groupadd -g {{.Gid}} {{.ContainerUser}}\n" +
	"useradd -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}\n" +
	"{{ else }}\n" +
	"old_id=`id -u {{.ContainerUser}}`\n" +
	"old_gid=`id -g {{.ContainerUser}}`\n" +
	"/usr/sbin/usermod {{.ContainerUser}} --uid {{.Uid}}\n" +
	"/usr/sbin/groupmod {{.ContainerUser}} --gid {{.Gid}}\n" +
	"for i in `find / -uid ${old_id}`; do /usr/bin/chgrp -R {{.Uid}} $i; done\n" +
	"for i in `find / -gid ${old_gid}`; do /usr/bin/chgrp -R {{.Gid}} $i; done\n" +
	"{{ end }}\n" +
	"{{ if .HasVolumes }}\n" +
	"chown -R {{.Uid}}:{{.Gid}} {{.Volumes}}\n" +
	"{{ end }}\n" +
	"exec su {{.ContainerUser}} -c -- {{.Command}}\n"))
