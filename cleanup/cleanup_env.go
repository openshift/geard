package cleanup

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
)

type EnvCleanup struct {
	envPath string
	homePath  string
}

func init() {
	envPath := filepath.Join(config.ContainerBasePath(), "env", "contents")
	homePath := filepath.Join(config.ContainerBasePath(), "home")

	AddCleaner(&EnvCleanup{envPath: envPath, homePath: homePath})
}

// Remove any orphaned container home directories
func (r *EnvCleanup) Clean(ctx *CleanerContext) {
	ctx.LogInfo.Println("--- ENV DIRECTORY REPAIR ---")
	ctx.LogInfo.Printf("envPath %s", r.envPath)
	ctx.LogInfo.Printf("system envPath %s", filepath.Join(config.ContainerBasePath(), "env", "contents"))

	indices, err := ioutil.ReadDir(r.envPath)
	if nil != err {
		ctx.LogError.Printf("Failed to obtain listing from %s: %v", r.envPath, err)
		return
	}

	for _, index := range indices {
		if !index.IsDir() {
			continue
		}

		// Remove any env variable file that is not associated with a service
		indexPath := filepath.Join(r.envPath, index.Name())
		entries, err := ioutil.ReadDir(indexPath)
		if nil != err {
			ctx.LogError.Printf("Failed to read indexDirectory %s: %v", indexPath, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			id, err := containers.NewIdentifier(entry.Name())
			if nil != err {
				ctx.LogError.Printf("%s is not a valid identifier: %v", entry.Name(), err)
				continue
			}

			if fileExist(id.BaseHomePath()) {
				continue
			}

			path := filepath.Join(r.envPath, index.Name(), entry.Name())
			ctx.LogInfo.Printf("Removing orphaned env file %s", path)
			if ctx.DryRun {
				continue
			}

			if err := os.Remove(path); nil != err {
				ctx.LogError.Printf("Failed to delete directory %s: %v", path, err)
			}
		}

		// Remove any index directory that no longer contains env variable files
		entries, err = ioutil.ReadDir(indexPath)
		if nil != err {
			ctx.LogError.Printf("Failed to obtain listing from %s: %v", r.envPath, err)
			continue
		}

		if 0 == len(entries) {
			ctx.LogInfo.Printf("Cleaning up env contents index %s", indexPath)
			if ctx.DryRun {
				continue
			}

			if err := os.RemoveAll(indexPath); nil != err {
				ctx.LogError.Printf("Failed to delete directory %s: %v", indexPath, err)
			}
		}
	}
}
