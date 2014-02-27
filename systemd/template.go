package systemd

import (
	"text/template"
)

type TargetUnit struct {
	Name     string
	WantedBy string
}

var TargetUnitTemplate = template.Must(template.New("unit.target").Parse(`
[Unit]
Description=Gear target {{.Name}}

[Install]
WantedBy={{.WantedBy}}
`))

type SliceUnit struct {
	Name   string
	Parent string
}

var SliceUnitTemplate = template.Must(template.New("unit.slice").Parse(`
[Unit]
Description=Gear slice {{.Name}}

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit=512M
{{ if .Parent }}Slice={{.Parent}}{{ end }}

[Install]
WantedBy=gear.target,gear-active.target
`))
