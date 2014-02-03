package geard

import (
	"fmt"
	"path/filepath"
)

var basePath = "/var/lib/gears"

func PathForContainerUnit(id string) string {
	return filepath.Join(basePath, "units", fmt.Sprintf("gear-%s.service", id))
}
