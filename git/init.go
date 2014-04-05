package git

import (
	"github.com/openshift/geard/systemd"
	"log"
)

func InitializeData() error {
	if err := initializeTargets(); err != nil {
		log.Fatal(err)
		return err
	}
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

func initializeTargets() error {
	return systemd.InitializeSystemdFile(systemd.TargetType, "githost", TargetGitTemplate, nil, false)
}

func initializeSlices() error {
	return systemd.InitializeSystemdFile(systemd.SliceType, "githost", SliceGitTemplate, nil, false)
}

func initializeGitHost() error {
	return systemd.InitializeSystemdFile(systemd.UnitType, "githost", UnitGitHostTemplate, nil, true)
}
