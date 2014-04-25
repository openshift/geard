package http

import (
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
)

// All registered extensions
var extensions []HttpExtension

type HttpExtension interface {
	Routes() []HttpJobHandler
	RequestFor(job jobs.Job) transport.TransportRequest
}

// Register an extension to this server during init() or startup
func AddHttpExtension(extension HttpExtension) {
	extensions = append(extensions, extension)
}
