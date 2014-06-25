// +build linux

package linux

import (
	"log"
	"path/filepath"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/systemd"
	rjobs "github.com/openshift/geard/router/jobs"
)

const hostServiceName = "geard-router"

func init() {
	// Bind mounted into the router
	config.AddRequiredDirectory(0755, filepath.Join(config.ContainerBasePath(), "router"))
}

func InitializeServices() error {
	// Using systemd to start the router automatically is disabled for now.
        return nil
	

	if err := initializeSlices(); err != nil {
		log.Fatal(err)
		return err
	}
	if err := initializeRouter(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func initializeSlices() error {
	return systemd.InitializeSystemdFile(systemd.SliceType, hostServiceName, rjobs.SliceRouterTemplate, nil, false)
}

func initializeRouter() error {
	if err := systemd.InitializeSystemdFile(systemd.UnitType, hostServiceName, rjobs.UnitRouterTemplate, nil, false); err != nil {
		return err
	}
	systemd.IsUnitProperty(systemd.Connection(), hostServiceName+".service", func(p map[string]interface{}) bool {
		switch p["ActiveState"] {
		case "active":
			break
		case "activating":
			log.Printf("The Router host service '" + hostServiceName + "' is starting - routing tasks will not be available until it completes")
		default:
			log.Printf("The Router host service '" + hostServiceName + "' is not started - router operations will not be available")
		}
		return true
	})
	return nil
}
