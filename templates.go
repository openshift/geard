package geard

import (
	"text/template"
)

type containerUnit struct {
	Gear     Identifier
	Image    string
	PortSpec string
	Slice    string
}

var containerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Gear container {{.Gear}}

[Service]
Type=simple
Slice={{.Slice}}
ExecStart=/usr/bin/docker run -name "gear-{{.Gear}}" -volumes-from "gear-{{.Gear}}" -a stdout -a stderr {{.PortSpec}} -rm "{{.Image}}"

[Install]
WantedBy=gear.target
`))

type sliceUnit struct {
	Name   string
	Parent string
}

var sliceUnitTemplate = template.Must(template.New("unit.slice").Parse(`
[Unit]
Description=Gear slice {{.Name}}

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit=512M
{{ if .Parent }}Slice={{.Parent}}{{ end }}

[Install]
WantedBy=gear.target
`))
