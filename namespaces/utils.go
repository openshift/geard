package namespaces

import (
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"os"
	"strings"
)

func addEnvIfNotSet(container *libcontainer.Container, key, value string) {
	jv := fmt.Sprintf("%s=%s", key, value)
	if len(container.Command.Env) == 0 {
		container.Command.Env = []string{jv}
		return
	}

	for _, v := range container.Command.Env {
		parts := strings.Split(v, "=")
		if parts[0] == key {
			return
		}
	}
	container.Command.Env = append(container.Command.Env, jv)
}

// print and error to stderr and exit(1)
func writeError(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
}
