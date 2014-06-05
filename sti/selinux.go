package sti

import "os/exec"

func selinuxEnabled() bool {
	path, err := exec.LookPath("selinuxenabled")
	if err == nil {
		cmd := exec.Command(path)
		err = cmd.Run()
		if err == nil {
			return true
		}
	}

	return false
}

func chcon(label, path string) error {
	if selinuxEnabled() {
		chconPath, err := exec.LookPath("chcon")
		if err == nil {
			chconCmd := exec.Command(chconPath, label, path)
			err = chconCmd.Run()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
