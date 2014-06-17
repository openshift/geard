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

func chcon(label, path string, recursive bool) error {
	if selinuxEnabled() {
		chconPath, err := exec.LookPath("chcon")
		if err == nil {
			var chconCmd *exec.Cmd
			if recursive {
				chconCmd = exec.Command(chconPath, "-R", label, path)
			} else {
				chconCmd = exec.Command(chconPath, label, path)
			}

			err = chconCmd.Run()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
