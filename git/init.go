package git

import (
	"github.com/openshift/geard/systemd"
	"log"
)

const hostServiceName = "geard-githost"

func InitializeData() error {
	if err := initializeSlices(); err != nil {
		log.Fatal(err)
		return err
	}
	if err := initializeGitHost(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func initializeSlices() error {
	return systemd.InitializeSystemdFile(systemd.SliceType, hostServiceName, SliceGitTemplate, nil, false)
}

func initializeGitHost() error {
	if err := systemd.InitializeSystemdFile(systemd.UnitType, hostServiceName, UnitGitHostTemplate, nil, false); err != nil {
		return err
	}
	systemd.IsUnitProperty(systemd.Connection(), hostServiceName+".service", func(p map[string]interface{}) bool {
		switch p["ActiveState"] {
		case "active":
			break
		case "activating":
			log.Printf("The Git host service '" + hostServiceName + "' is starting - repository tasks will not be available until it completes")
		default:
			log.Printf("The Git host service '" + hostServiceName + "' is not started - Git repository operations will not be available")
		}
		return true
	})
	return nil
}
