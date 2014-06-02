package sti

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
)

// Usage processes a build request by starting the container and executing
// the assemble script with a "-h" argument to print usage information
// for the script.
func Usage(req BuildRequest) (*BuildResult, error) {
	h, err := newHandler(req.Request)
	if err != nil {
		return nil, err
	}

	if req.WorkingDir == "tempdir" {
		var err error
		req.WorkingDir, err = ioutil.TempDir("", "sti")
		if err != nil {
			return nil, fmt.Errorf("Error creating temporary directory '%s': %s\n", req.WorkingDir, err.Error())
		}
		defer RemoveDirectory(req.WorkingDir, req.Verbose)
	}

	dirs := []string{"scripts", "defaultScripts"}
	for _, v := range dirs {
		err := os.Mkdir(filepath.Join(req.WorkingDir, v), 0700)
		if err != nil {
			return nil, err
		}
	}

	if req.ScriptsUrl != "" {
		url, _ := url.Parse(req.ScriptsUrl + "/" + "assemble")
		downloadFile(url, req.WorkingDir+"/scripts/assemble", h.verbose)
	}

	defaultUrl, err := h.getDefaultUrl(req, req.BaseImage)
	if err != nil {
		return nil, err
	}
	if defaultUrl != "" {
		url, _ := url.Parse(defaultUrl + "/" + "assemble")
		downloadFile(url, req.WorkingDir+"/defaultScripts/assemble", h.verbose)
	}

	return h.buildDeployableImage(req, req.BaseImage, req.WorkingDir, false)
}
