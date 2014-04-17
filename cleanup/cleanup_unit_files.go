package cleanup

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/systemd"
	"os"
	"path/filepath"
	"time"
)

type UnitFilesCleanup struct {
	unusedFor time.Duration
}

func init() {
	AddCleaner(&UnitFilesCleanup{unusedFor: 24 * time.Hour})
}

// Remove all files from the directory excluding the specified file.
func removeFilesExcluding(excFileName string, definitionsDir string, unusedFor time.Duration, ctx *CleanerContext) error {
	filepath.Walk(definitionsDir, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			ctx.LogInfo.Printf("cleanup_unit_files: error accessing %s %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Base(path) == excFileName {
			return nil
		}

		var fi os.FileInfo

		if fi, err = os.Stat(path); err != nil {
			ctx.LogError.Printf("Could not file information for %s", path)
			return nil
		}

		if time.Since(fi.ModTime()) > unusedFor {
			if ctx.DryRun {
				ctx.LogInfo.Printf("%s could be removed as it is unused.", path)
			} else {
				ctx.LogInfo.Printf("Removing unused file %s.", path)
				if er := os.Remove(path); er != nil {
					ctx.LogError.Printf("Failed to remove %s: %v", path, er)
				}
			}
		}

		return nil
	})

	return nil
}

// Removes unused definition files by checking what definition files
// are actually in use in the service file.
func (r *UnitFilesCleanup) Clean(ctx *CleanerContext) {
	if !ctx.Repair {
		return
	}

	ctx.LogInfo.Println("--- UNIT FILES REPAIR ---")

	unitsPath := filepath.Join(config.ContainerBasePath(), "units")

	filepath.Walk(unitsPath, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			ctx.LogError.Printf("repair_unit_files: Can't read %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".service" {
			return nil
		}

		props, er := systemd.GetUnitFileProperties(path)
		if er != nil {
			ctx.LogError.Println("Failed to get unit file properties")
			return er
		}

		// X-ContainerRequestId property has the name of the definition file in use.
		currDefinitionFile, ok := props["X-ContainerRequestId"]
		if !ok {
			return nil
		}

		containerId, ok := props["X-ContainerId"]
		if !ok {
			return nil
		}

		definitionsDirPath := filepath.Join(filepath.Dir(path), containerId)
		removeFilesExcluding(currDefinitionFile, definitionsDirPath, r.unusedFor, ctx)

		// TODO: Also remove empty directories.
		// TODO: Validate the ports and other information in the systemd file.

		return nil
	})
}
