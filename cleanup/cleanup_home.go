package cleanup

import (
	"os"
	"path"
	"path/filepath"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
)

type HomeCleanup struct {
	homePath  string
	unitsPath string
}

func init() {
	homePath := filepath.Join(config.ContainerBasePath(), "home")
	AddCleaner(&HomeCleanup{homePath: homePath})
}

// Remove any orphaned container home directories
func (r *HomeCleanup) Clean(ctx *CleanerContext) {
	ctx.LogInfo.Println("--- HOME DIRECTORY REPAIR ---")

	indexPath := filepath.Join(r.homePath, "*")
	indices, err := filepath.Glob(indexPath)
	if nil != err {
		ctx.LogError.Printf("No indices found in %s directory: %v", indexPath, err)
		return
	}

	for _, index := range indices {
		servicePath := filepath.Join(index, "*")
		services, err := filepath.Glob(servicePath)
		if nil != err {
			ctx.LogError.Printf("Failed to find any services in %s: %v", servicePath, err)
			continue
		}

		for _, service := range services {
			id, err := containers.NewIdentifier(path.Base(service))
			if nil != err {
				ctx.LogError.Printf("Failed to convert %s to Identifier: %v", path.Base(service), err)
				continue
			}

			if fileExist(id.UnitPathFor()) {
				continue
			}

			ctx.LogInfo.Printf("Removing orphaned home directory %s there is no associated unit file", service)
			if ctx.DryRun {
				continue
			}

			// Remove orphaned home directory
			err = os.RemoveAll(service)
			if nil != err {
				ctx.LogError.Printf("Failed removing directory %s: %v", service, err)
			}
		}
	}
}

func fileExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// Any other errors reported as existing for safety
	return true
}
