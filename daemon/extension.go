package daemon

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/openshift/geard/dispatcher"
)

type DaemonExtension interface {
	// Returns the list of supported schemes
	Schemes() []string
	// Addr is a URI with at least a protocol and host. Should not
	// return as long as the server is active.
	Start(addr string, dispatcher *dispatcher.Dispatcher, done chan<- bool) error
}

// All registered extensions
var extensions []DaemonExtension

// Register an extension to this server during init() or startup
func AddDaemonExtension(extension DaemonExtension) {
	extensions = append(extensions, extension)
}

func DaemonExtensions() []DaemonExtension {
	return extensions[:]
}

func DaemonExtensionFor(addr string) (DaemonExtension, error) {
	addrUrl, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("the specified address is not a valid URL: %s", err.Error())
	}
	for i := range extensions {
		if contains(extensions[i].Schemes(), addrUrl.Scheme) {
			return extensions[i], nil
		}
	}
	protocols := []string{}
	return nil, fmt.Errorf("specify a URL to listen on from the following: %s", strings.Join(protocols, ", "))
}

func contains(in []string, find string) bool {
	for i := range in {
		if in[i] == find {
			return true
		}
	}
	return false
}
