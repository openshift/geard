package containers

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/port"
	"text/template"
)

type ContainerUnit struct {
	Id       Identifier
	Image    string
	PortSpec string
	RunSpec  string
	Slice    string
	Isolate  bool
	User     string
	ReqId    string

	HomeDir         string
	EnvironmentPath string
	ExecutablePath  string
	IncludePath     string

	PortPairs            port.PortPairs
	SocketUnitName       string
	SocketActivationType string

	DockerFeatures config.DockerFeatures
}

var ContainerUnitTemplate = template.Must(template.New("unit.service").Parse(`
{{define "COMMON_UNIT"}}
[Unit]
Description=Container {{.Id}}
{{end}}

{{define "COMMON_SERVICE"}}
[Service]
Type=simple
TimeoutStartSec=5m
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
{{end}}

{{define "COMMON_CONTAINER"}}
[Install]
WantedBy=container.target

# Container information
X-ContainerId={{.Id}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
X-ContainerType={{ if .Isolate }}isolated{{ else }}simple{{ end }}
{{range .PortPairs}}X-PortMapping={{.Internal}}:{{.External}}
{{end}}
{{end}}

{{/* A unit that lets docker own the container processes and only integrates via API */}}
{{define "SIMPLE"}}
{{template "COMMON_UNIT" .}}
{{template "COMMON_SERVICE" .}}
# Create data container
ExecStartPre=/bin/sh -c '/usr/bin/docker inspect --format="Reusing {{"{{.ID}}"}}" "{{.Id}}-data" || exec docker run --name "{{.Id}}-data" --volumes-from "{{.Id}}-data" --entrypoint true "{{.Image}}"'
ExecStartPre=-/usr/bin/docker rm "{{.Id}}"
{{ if .Isolate }}# Initialize user and volumes
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"{{ end }}
ExecStart=/usr/bin/docker run --rm --name "{{.Id}}" \
          --volumes-from "{{.Id}}-data" \
          {{ if and .EnvironmentPath .DockerFeatures.EnvironmentFile }}--env-file "{{ .EnvironmentPath }}"{{ end }} \
          -a stdout -a stderr {{.PortSpec}} {{.RunSpec}} \
          {{ if .Isolate }} -v {{.HomeDir}}/container-cmd.sh:/.container.cmd:ro -v {{.HomeDir}}/container-init.sh:/.container.init:ro -u root {{end}} \
          "{{.Image}}" {{ if .Isolate }} /.container.init {{ end }}
# Set links (requires container have a name)
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"
ExecReload=-/usr/bin/docker stop "{{.Id}}"
ExecReload=-/usr/bin/docker rm "{{.Id}}"
ExecStop=-/usr/bin/docker stop "{{.Id}}"
{{template "COMMON_CONTAINER" .}}
{{end}}

{{/* A unit that uses Docker with the 'foreground' flag to run an image under the current context */}}
{{define "FOREGROUND"}}
{{template "COMMON_UNIT" .}}
{{template "COMMON_SERVICE" .}}
# Create data container
ExecStartPre=/bin/sh -c '/usr/bin/docker inspect --format="Reusing {{"{{.ID}}"}}" "{{.Id}}-data" || exec docker run --name "{{.Id}}-data" --volumes-from "{{.Id}}-data" --entrypoint true "{{.Image}}"'
ExecStartPre=-/usr/bin/docker rm "{{.Id}}"
{{ if .Isolate }}# Initialize user and volumes
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"{{ end }}
ExecStart=/usr/bin/docker run --rm --foreground \
          {{ if and .EnvironmentPath .DockerFeatures.EnvironmentFile }}--env-file "{{ .EnvironmentPath }}"{{ end }} \
          {{.PortSpec}} {{.RunSpec}} \
          --name "{{.Id}}" --volumes-from "{{.Id}}-data" \
          {{ if .Isolate }} -v {{.HomeDir}}/container-cmd.sh:/.container.cmd:ro -v {{.HomeDir}}/container-init.sh:/.container.init:ro -u root {{end}} \
          "{{.Image}}" {{ if .Isolate }} /.container.init {{ end }}
# Set links (requires container have a name)
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"
{{template "COMMON_CONTAINER" .}}
{{end}}

{{/* A unit that exposes socket activation and process isolation */}}
{{define "SOCKETACTIVATED"}}
{{template "COMMON_UNIT" .}}
BindsTo={{.SocketUnitName}}

{{template "COMMON_SERVICE" .}}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            --name "{{.Id}}" \
            --volumes-from "{{.Id}}" \
            {{ if and .EnvironmentPath .DockerFeatures.EnvironmentFile }}--env-file "{{ .EnvironmentPath }}"{{ end }} \
            -a stdout -a stderr {{.RunSpec}} \
            --env LISTEN_FDS \
            -v {{.HomeDir}}/container-init.sh:/.container.init:ro \
            -v {{.HomeDir}}/container-cmd.sh:/.container.cmd:ro \
            -v /usr/sbin/systemd-socket-proxyd:/usr/sbin/systemd-socket-proxyd:ro \
            -u root -f --rm \
            "{{.Image}}" /.container.init
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"
{{template "COMMON_CONTAINER" .}}
X-SocketActivated={{.SocketActivationType}}
{{end}}

{{/* Run DEFAULT */}}
{{template "SIMPLE" .}}
`))

var ContainerSocketTemplate = template.Must(template.New("unit.socket").Parse(`
[Unit]
Description=Container socket {{.Id}}

[Socket]
{{range .PortPairs}}ListenStream={{.External}}
{{end}}

[Install]
WantedBy=container-sockets.target
`))

type ContainerInitScript struct {
	CreateUser     bool
	ContainerUser  string
	Uid            string
	Gid            string
	Command        string
	HasVolumes     bool
	Volumes        string
	PortPairs      port.PortPairs
	UseSocketProxy bool
}

var ContainerInitTemplate = template.Must(template.New("container-init.sh").Parse(`#!/bin/bash
{{ if .CreateUser }}
groupadd -g {{.Gid}} {{.ContainerUser}}
useradd -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}
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
{{ if .UseSocketProxy }}
bash -c 'LISTEN_PID=$$ exec /usr/sbin/systemd-socket-proxyd {{ range .PortPairs }}127.0.0.1:{{ .Internal }}{{ end }}' &
{{ end }}
exec su {{.ContainerUser}} -s /bin/bash -c /.container.cmd
`))

var ContainerCmdTemplate = template.Must(template.New("container-cmd.sh").Parse(`#!/bin/bash
exec {{.Command}}
`))

type OutboundNetworkIptables struct {
	// The IP address for inbound source NAT
	SourceAddr string
	// The local IP and port to connect to
	LocalAddr string
	LocalPort port.Port
	// The remote IP and port to connect to
	DestAddr string
	DestPort port.Port
}

var OutboundNetworkIptablesTemplate = template.Must(template.New("outbound_network.iptables").Parse(`
-A PREROUTING -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A OUTPUT -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A POSTROUTING -o eth0 -j SNAT --to-source {{.SourceAddr}}
`))

type TargetUnit struct {
	Name     string
	WantedBy string
}

var TargetUnitTemplate = template.Must(template.New("unit.target").Parse(`
[Unit]
Description=Container target {{.Name}}

[Install]
WantedBy={{.WantedBy}}
`))

type SliceUnit struct {
	Name   string
	Parent string
}

var SliceUnitTemplate = template.Must(template.New("unit.slice").Parse(`
[Unit]
Description=Container slice {{.Name}}

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit=512M
{{ if .Parent }}Slice={{.Parent}}{{ end }}

[Install]
WantedBy=container.target container-active.target
`))
