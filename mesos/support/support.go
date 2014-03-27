// +build mesos

package support

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/utils"

	"os"
	"path"
)

func HasBinaries() bool {
	for _, b := range []string{
		path.Join(config.ContainerBasePath(), "bin", "gear-mesos-executor"),
	} {
		if _, err := os.Stat(b); err != nil {
			return false
		}
	}
	return true
}

func InitializeBinaries() error {
	srcDir := path.Dir(os.Args[0])
	destDir := path.Join(config.ContainerBasePath(), "bin")
	if err := utils.CopyBinary(path.Join(srcDir, "gear-mesos-executor"), path.Join(destDir, "gear-mesos-executor"), false); err != nil {
		return err
	}
	return nil
}
