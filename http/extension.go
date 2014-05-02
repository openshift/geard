package http

import (
	"github.com/openshift/geard/jobs"
)

// All registered extensions
var extensions []HttpExtension

type HttpExtension interface {
	Routes() []HttpJobHandler
	HttpJobFor(job jobs.Job) (RemoteExecutable, error)
}

// Register an extension to this server during init() or startup
func AddHttpExtension(extension HttpExtension) {
	extensions = append(extensions, extension)
}
