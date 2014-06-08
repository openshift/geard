package http

import (
	"github.com/openshift/geard/http/client"
)

// All registered extensions
var extensions []HttpExtension

type HttpExtension interface {
	Routes() ExtensionMap
	HttpJobFor(request interface{}) (client.RemoteExecutable, error)
}

// Register an extension to this server during init() or startup
func AddHttpExtension(extension HttpExtension) {
	client.AddHttpExtension(extension)
	extensions = append(extensions, extension)
}
