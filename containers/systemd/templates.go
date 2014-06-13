package systemd

import (
	"text/template"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
)

type ContainerUnit struct {
	// Data about the container
	Id              containers.Identifier
	Image           string
	Slice           string
	Isolate         bool
	User            string
	ReqId           string
	EnvironmentPath string
	Cmd             string

	// Arguments for docker run
	RunSpec string

	ExecutablePath string

	PortPairs            port.PortPairs
	SocketUnitName       string
	SocketActivationType string

	DockerFeatures config.DockerFeatures
}

// A unit for running a docker container
var ContainerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Container {{.Id}}
{{ if .SocketUnitName }}BindsTo={{.SocketUnitName}}{{ end }}

[Service]
Type=simple
TimeoutStartSec=5m
{{ if .Slice }}Slice={{.Slice}}{{ end }}
{{ if .EnvironmentPath }}EnvironmentFile={{.EnvironmentPath}}{{ end }}
ExecStartPre={{.ExecutablePath}} init-container {{ .RunSpec }}
ExecStart=/usr/bin/docker start "{{.Id}}"
ExecStartPost={{.ExecutablePath}} link-container "{{.Id}}"
ExecStop=-/usr/bin/docker stop "{{.Id}}"

[Install]
WantedBy=container.target

# Container information
X-ContainerId={{.Id}}
X-ContainerCmd={{.Cmd}}
X-ContainerImage={{.Image}}
X-ContainerUserId={{.User}}
X-ContainerRequestId={{.ReqId}}
X-ContainerType={{ if .Isolate }}isolated{{ else }}simple{{ end }}
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
	Name        string
	Parent      string
	MemoryLimit string
}

var SliceUnitTemplate = template.Must(template.New("unit.slice").Parse(`
[Unit]
Description=Container slice {{.Name}}

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit={{.MemoryLimit}}
{{ if .Parent }}Slice={{.Parent}}{{ end }}

[Install]
WantedBy=container.target container-active.target
`))
