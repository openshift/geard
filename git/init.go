package git

import (
	"github.com/openshift/geard/systemd"
	"log"
)

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
	return systemd.InitializeSystemdFile(systemd.SliceType, "geard-githost", SliceGitTemplate, nil, false)
}

func initializeGitHost() error {
	return systemd.InitializeSystemdFile(systemd.UnitType, "geard-githost", UnitGitHostTemplate, nil, true)
}
