package cleanup

import (
	"os"
	"path/filepath"
	"time"

	"github.com/openshift/geard/config"
)

type PortsCleanup struct {
	unusedFor time.Duration
}

func init() {
	AddCleaner(&PortsCleanup{unusedFor: 24 * time.Hour})
}

// Remove port allocations that don't point to systemd definition files.
func (r *PortsCleanup) Clean(ctx *CleanerContext) {
	ctx.LogInfo.Println("--- PORTS CLEANUP ---")

	portsPath := filepath.Join(config.ContainerBasePath(), "ports", "interfaces")

	filepath.Walk(portsPath, func(path string, fi os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			ctx.LogError.Printf("Can't read %s: %v", path, err)
			return nil
		}
		if fi.IsDir() {
			return nil
		}

		if fi.Mode() & os.ModeSymlink == os.ModeSymlink {
			unitPath, err := os.Readlink(path)
			if err != nil {
				ctx.LogError.Printf("Failed to read the link: %v", err)
				return nil
			}

			if _, err := os.Stat(unitPath); os.IsNotExist(err) {
				if ctx.DryRun {
					ctx.LogInfo.Printf("Port %v could be recovered as it does not point to a definition file", path)
				} else {
					ctx.LogInfo.Printf("Recovering port %v as it does point to a definition file.", path)
					if err = os.Remove(path); err != nil {
						ctx.LogError.Printf("Failed to remove %s: %v", path, err)
					}
				}
				return nil
			}
		}

		return nil
	})
}
