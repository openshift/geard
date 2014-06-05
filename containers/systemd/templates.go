package systemd

import (
	"text/template"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
)

type ContainerUnit struct {
	Id       containers.Identifier
	Image    string
	PortSpec string
	RunSpec  string
	Slice    string
	Isolate  bool
	User     string
	ReqId    string

	HomeDir         string
	RunDir          string
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
          {{ if .Isolate }} -v {{.RunDir}}/container-cmd.sh:/.container.cmd:ro -v {{.RunDir}}/container-init.sh:/.container.init:ro -u root {{end}} \
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
          {{ if .Isolate }} -v {{.RunDir}}/container-cmd.sh:/.container.cmd:ro -v {{.RunDir}}/container-init.sh:/.container.init:ro -u root {{end}} \
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
            -v {{.RunDir}}/container-init.sh:/.container.init:ro \
            -v {{.RunDir}}/container-cmd.sh:/.container.cmd:ro \
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
	Name         string
	Parent       string
	MemoryLimit  string
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
