package containers

import (
	"text/template"
)

type ContainerUnit struct {
	Id       Identifier
	Image    string
	PortSpec string
	Slice    string
	Isolate  bool
	User     string
	ReqId    string

	HomeDir         string
	EnvironmentPath string
	ExecutablePath  string
	IncludePath     string

	PortPairs            PortPairs
	SocketUnitName       string
	SocketActivationType string
}

var SimpleContainerUnitTemplate = template.Must(template.New("simple_unit.service").Parse(`
[Unit]
Description=Container {{.Id}}

[Service]
Type=simple
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
ExecStart=/bin/sh -c '/usr/bin/docker inspect -format="Reusing {{"{{.ID}}"}}" "{{.Id}}" 2>/dev/null && \
                      exec /usr/bin/docker start -a "{{.Id}}" || \
                      exec /usr/bin/docker run -name "{{.Id}}" -volumes-from "{{.Id}}" -a stdout -a stderr {{.PortSpec}} "{{.Image}}"'
ExecReload=/usr/bin/docker stop "{{.Id}}"
ExecReload=/usr/bin/docker rm "{{.Id}}"

TimeoutStartSec=5m

{{ if .IncludePath }}.include {{.IncludePath}} {{ end }}

# Container information
X-ContainerId={{.Id}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
{{range .PortPairs}}X-PortMapping={{.Internal}}:{{.External}}
{{end}}
`))

var ContainerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Container {{.Id}}

[Service]
Type=simple
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
{{ if .Isolate }}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            -name "{{.Id}}" -rm \
            -volumes-from "{{.Id}}" \
            -a stdout -a stderr {{.PortSpec}} \
            -v {{.HomeDir}}/container-init.sh:/.container.init:ro -u root \
            "{{.Image}}" /.container.init
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"
{{else}}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            -name "{{.Id}}" -rm \
            -volumes-from "{{.Id}}" \
            -a stdout -a stderr {{.PortSpec}} \
            "{{.Image}}"
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"
{{ end }}

TimeoutStartSec=5m

{{ if .IncludePath }}.include {{.IncludePath}} {{ end }}

# Container information
X-ContainerId={{.Id}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
X-ContainerType={{ if .Isolate }}isolated{{ else }}simple{{ end }}
X-SocketActivation=disabled
{{range .PortPairs}}X-PortMapping={{.Internal}}:{{.External}}
{{end}}
`))

var ContainerSocketActivatedUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Container {{.Id}}
BindsTo={{.SocketUnitName}}

[Service]
Type=simple
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
ExecStartPre={{.ExecutablePath}} init --pre "{{.Id}}" "{{.Image}}"
ExecStart=/usr/bin/docker run \
            -name "{{.Id}}" \
            -volumes-from "{{.Id}}" \
            -a stdout -a stderr \
            --env LISTEN_FDS \
            -v {{.HomeDir}}/container-init.sh:/.container.init:ro \
            -v /usr/sbin/systemd-socket-proxyd:/usr/sbin/systemd-socket-proxyd:ro \
            -u root -f -rm \
            "{{.Image}}" /.container.init
ExecStartPost=-{{.ExecutablePath}} init --post "{{.Id}}" "{{.Image}}"

TimeoutStartSec=5m

{{ if .IncludePath }}.include {{.IncludePath}} {{ end }}

# Container information
X-ContainerId={{.Id}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
X-ContainerType=isolated
X-SocketActivated={{.SocketActivationType}}
{{range .PortPairs}}X-PortMapping={{.Internal}}:{{.External}}
{{end}}
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
	PortPairs      PortPairs
	UseSocketProxy bool
}

var ContainerInitTemplate = template.Must(template.New("container-init.sh").Parse(`#!/bin/bash
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
{{ if .UseSocketProxy }}
bash -c 'LISTEN_PID=$$ exec /usr/sbin/systemd-socket-proxyd {{ range .PortPairs }}127.0.0.1:{{ .Internal }}{{ end }}' &
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
WantedBy=container.target,container-active.target
`))
