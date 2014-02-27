package gears

import (
	"text/template"
)

type ContainerUnit struct {
	Gear             Identifier
	Image            string
	PortSpec         string
	Slice            string
	Isolate          bool
	User             string
	ReqId            string
	HomeDir          string
	EnvironmentPath  string
	ExecutablePath   string
	IncludePath      string
	DynamicWantsPath string
}

var SimpleContainerUnitTemplate = template.Must(template.New("simple_unit.service").Parse(`
[Unit]
Description=Gear container {{.Gear}}

[Service]
Type=simple
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
ExecStart=/bin/sh -c '/usr/bin/docker inspect -format="Reusing {{"{{.ID}}"}}" "gear-{{.Gear}}" 2>/dev/null && \
                      exec /usr/bin/docker start -a "gear-{{.Gear}}" || \
                      exec /usr/bin/docker run -name "gear-{{.Gear}}" -volumes-from "gear-{{.Gear}}" -a stdout -a stderr {{.PortSpec}} "{{.Image}}"'
ExecReload=/usr/bin/docker stop "gear-{{.Gear}}"
ExecReload=/usr/bin/docker rm "gear-{{.Gear}}"

{{ if .IncludePath }}.include {{.IncludePath}} {{ end }}

# Gear information
X-GearId={{.Gear}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
`))

var ContainerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Gear container {{.Gear}}

[Service]
Type=simple
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
{{ if .Isolate }}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Gear}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            -name "gear-{{.Gear}}" -rm \
            -volumes-from "gear-{{.Gear}}" \
            -a stdout -a stderr {{.PortSpec}} \
            -v {{.HomeDir}}/gear-init.sh:/.gear.init:ro -u root \
            "{{.Image}}" /.gear.init
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Gear}}" "{{.Image}}"
{{else}}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Gear}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            -name "gear-{{.Gear}}" -rm \
            -volumes-from "gear-{{.Gear}}" \
            -a stdout -a stderr {{.PortSpec}} \
            "{{.Image}}"
{{ end }}

{{ if .IncludePath }}.include {{.IncludePath}} {{ end }}

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

var GearInitTemplate = template.Must(template.New("gear-init.sh").Parse(`#!/bin/bash
{{ if .CreateUser }}
groupadd -g {{.Gid}} {{.ContainerUser}}
useradd -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}
{{ else }}
old_id=$(id -u {{.ContainerUser}})
old_gid=$(id -g {{.ContainerUser}})
/usr/sbin/usermod {{.ContainerUser}} --uid {{.Uid}}
/usr/sbin/groupmod {{.ContainerUser}} --gid {{.Gid}}
for i in $(find / -uid ${old_id}); do /usr/bin/chgrp -R {{.Uid}} $i; done
for i in $(find / -gid ${old_gid}); do /usr/bin/chgrp -R {{.Gid}} $i; done
{{ end }}
{{ if .HasVolumes }}
chown -R {{.Uid}}:{{.Gid}} {{.Volumes}}
{{ end }}
exec su {{.ContainerUser}} -c -- {{.Command}}
`))

type OutboundNetworkIptables struct {
	// The IP address for inbound source NAT
	SourceAddr string
	// The local IP and port to connect to
	LocalAddr string
	LocalPort Port
	// The remote IP and port to connect to
	DestAddr string
	DestPort Port
}

var OutboundNetworkIptablesTemplate = template.Must(template.New("outbound_network.iptables").Parse(`
*nat
-A PREROUTING -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A OUTPUT -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A POSTROUTING -o eth0 -j SNAT --to-source {{.SourceAddr}}
COMMIT
`))
