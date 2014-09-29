package sti

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
)

// Usage processes a build request by starting the container and executing
// the assemble script with a "-h" argument to print usage information
// for the script.
func Usage(req *STIRequest) error {
	h, err := newHandler(req)
	if err != nil {
		return err
	}

	h.request.workingDir, err = createWorkingDirectory()
	if err != nil {
		return err
	}
	if h.request.PreserveWorkingDir {
		log.Printf("Temporary directory '%s' will be saved, not deleted\n", h.request.workingDir)
	} else {
		defer removeDirectory(h.request.workingDir, h.request.Verbose)
	}

	dirs := []string{"upload/scripts", "downloads/scripts", "downloads/defaultScripts"}
	for _, v := range dirs {
		err := os.MkdirAll(filepath.Join(h.request.workingDir, v), 0700)
		if err != nil {
			return err
		}
	}

	if req.ScriptsUrl != "" {
		url, _ := url.Parse(req.ScriptsUrl + "/" + "usage")
		err = downloadFile(url, h.request.workingDir+"/downloads/scripts/usage", h.request.Verbose)
	}

	defaultUrl, err := h.getDefaultUrl()
	if err != nil {
		return err
	}
	if defaultUrl != "" {
		url, _ := url.Parse(defaultUrl + "/" + "usage")
		downloadFile(url, h.request.workingDir+"/downloads/defaultScripts/usage", h.request.Verbose)
	}

	h.request.usage = true
	_, _, err = h.buildInternal()
	return err
}
