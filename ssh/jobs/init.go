package jobs

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

const sshdConfigPath = "/etc/ssh/sshd_config"

// Makes sure that the ssh configuration has the required settings.
func checkSshdConfig() error {
	// Settings required for ssh to gear feature.
	settings := []string{
		"AuthorizedKeysCommand /usr/sbin/gear-auth-keys-command",
		"AuthorizedKeysCommandUser nobody",
	}

	allFound := true
	for _, setting := range settings {
		found, err := checkSettingExistsInFile(sshdConfigPath, setting)
		if err != nil {
			log.Println("Failed to check sshd config for settings: ", err)
			continue
		}
		if !found {
			allFound = false
			break
		}
	}

	if !allFound {
		log.Printf("SSH configuration does not have the required settings. Please make sure that the following two lines are present in %v", sshdConfigPath)
		for _, setting := range settings {
			log.Println(setting)
		}
		return fmt.Errorf("SSH configuration does not have required settings.")
	}

	return nil
}

// Check if setting matches any line in the file exactly.
func checkSettingExistsInFile(path string, setting string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if sc.Text() == setting {
			return true, nil
		}
	}

	return false, nil
}
