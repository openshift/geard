package jobs

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/systemd"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

type DeleteContainerRequest struct {
	Id containers.Identifier
}

func (j *DeleteContainerRequest) Execute(resp JobResponse) {
	unitName := j.Id.UnitNameFor()
	unitPath := j.Id.UnitPathFor()
	unitDefinitionsPath := j.Id.VersionedUnitsPathFor()
	idleFlagPath := j.Id.IdleUnitPathFor()
	socketUnitPath := j.Id.SocketUnitPathFor()
	homeDirPath := j.Id.BaseHomePath()
	runDirPath := j.Id.RunPathFor()
	networkLinksPath := j.Id.NetworkLinksPathFor()
	userName := j.Id.LoginFor()

	_, err := systemd.Connection().GetUnitProperties(unitName)
	switch {
	case systemd.IsNoSuchUnit(err):
		resp.Success(JobResponseOk)
		return
	case err != nil:
		resp.Failure(ErrDeleteContainerFailed)
		return
	}

	if err := systemd.Connection().StopUnitJob(unitName, "fail"); err != nil {
		log.Printf("delete_container: Unable to queue stop unit job: %v", err)
	}

	ports, err := containers.GetExistingPorts(j.Id)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("delete_container: Unable to read existing port definitions: %v", err)
		}
		ports = port.PortPairs{}
	}

	if err := port.ReleaseExternalPorts(ports); err != nil {
		log.Printf("delete_container: Unable to release ports: %v", err)
	}

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		resp.Failure(ErrDeleteContainerFailed)
		return
	}

	if err := os.Remove(idleFlagPath); err != nil && !os.IsNotExist(err) {
		resp.Failure(ErrDeleteContainerFailed)
		return
	}

	if err := j.Id.SetUnitStartOnBoot(false); err != nil {
		log.Printf("delete_container: Unable to clear unit boot state: %v", err)
	}

	if err := os.Remove(socketUnitPath); err != nil && !os.IsNotExist(err) {
		log.Printf("delete_container: Unable to remove socket unit path: %v", err)
	}

	if err := os.Remove(networkLinksPath); err != nil && !os.IsNotExist(err) {
		log.Printf("delete_container: Unable to remove network links file: %v", err)
	}

	if err := os.RemoveAll(unitDefinitionsPath); err != nil {
		log.Printf("delete_container: Unable to remove definitions for container: %v", err)
	}

	if err := os.RemoveAll(filepath.Dir(runDirPath)); err != nil {
		log.Printf("delete_container: Unable to remove run directory: %v", err)
	}

	_, err = user.Lookup(userName)
	if err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			log.Printf("delete_container: Unable to lookup user: %v", err)
		}
	} else {
		cmd := exec.Command("/usr/sbin/userdel", userName)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("delete_container: Failed to remove user: %v %v", err, out)
		}
	}

	if err := os.RemoveAll(filepath.Dir(homeDirPath)); err != nil {
		log.Printf("delete_container: Unable to remove home directory: %v", err)
	}

	if _, err := systemd.Connection().DisableUnitFiles([]string{unitPath, socketUnitPath}, false); err != nil {
		log.Printf("delete_container: Some units have not been disabled: %v", err)
	}

	if err := systemd.Connection().Reload(); err != nil {
		log.Printf("delete_container: Some units have not been disabled: %v", err)
	}

	resp.Success(JobResponseOk)
}
