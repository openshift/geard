package cleanup

import "os"

func fileExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// Any other errors reported as existing for safety
	return true
}

