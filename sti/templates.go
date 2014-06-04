package sti

import "text/template"

// Script used to initialize permissions on bind-mounts when a non-root user is specified by an image
var saveArtifactsInitTemplate = template.Must(template.New("sa-init.sh").Parse(`#!/bin/sh
chown -R {{.User}} /tmp/artifacts && chmod -R 775 /tmp/artifacts
chmod -R 755 /tmp/scripts
chmod -R 755 /tmp/defaultScripts
chmod -R 755 /tmp/src
exec su {{.User}} - -s /bin/sh -c {{.SaveArtifactsPath}}
`))

// Script used to initialize permissions on bind-mounts for a docker-run build (prepare call)
var buildTemplate = template.Must(template.New("build-init.sh").Parse(`#!/bin/sh
{{if eq .Usage false }}chmod -R 755 /tmp/src{{end}}
chmod -R 755 /tmp/scripts
chmod -R 755 /tmp/defaultScripts
{{if .Incremental}}chown -R {{.User}} /tmp/artifacts && chmod -R 775 /tmp/artifacts{{end}}
mkdir -p /opt/sti/bin
if [ -f {{.RunPath}} ]; then
	cp {{.RunPath}} /opt/sti/bin
fi

if [ -f {{.AssemblePath}} ]; then
	{{if .Usage}}
		exec su {{.User}} - -s /bin/sh -c "{{.AssemblePath}} -h"
	{{else}}
		exec su {{.User}} - -s /bin/sh -c {{.AssemblePath}}
	{{end}}
else
  echo "No assemble script supplied in ScriptsUrl argument, application source, or default url in the image."
fi
`))
