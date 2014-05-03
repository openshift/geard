/*
   Copyright 2014 Red Hat, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cleanup

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"os"
	"path"
	"path/filepath"
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

			unitPath := id.UnitPathFor()
			idleUnitPath := id.IdleUnitPathFor()

			if fileExist(unitPath) || fileExist(idleUnitPath) {
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
