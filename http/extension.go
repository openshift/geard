package http

// All registered extensions
var extensions []HttpExtension

type HttpExtension func() []HttpJobHandler

// Register an extension to this server during init() or startup
func AddHttpExtension(extension HttpExtension) {
	extensions = append(extensions, extension)
}
