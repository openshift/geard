package client

// All registered extensions
var extensions []HttpExtension

type HttpExtension interface {
	HttpJobFor(request interface{}) (RemoteExecutable, error)
}

// Register an extension to this server during init() or startup
func AddHttpExtension(extension HttpExtension) {
	extensions = append(extensions, extension)
}
