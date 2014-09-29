package sti

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// createTarUpload creates a tar file that contains the contents of the upload
// directory and returns the name of the tar file
func (h requestHandler) createTarUpload() (string, error) {
	tarFile, err := ioutil.TempFile(h.request.workingDir, "tar")
	if err != nil {
		return "", err
	}

	tarWriter := tar.NewWriter(tarFile)
	uploadDir := filepath.Join(h.request.workingDir, "upload")

	err = filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.Index(path, ".git") == -1 {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			tarName := filepath.Join("/tmp", path[len(uploadDir):])
			header.Name = tarName

			if h.request.Verbose {
				log.Printf("Adding to tar: %s as %s\n", path, tarName)
			}

			if err = tarWriter.WriteHeader(header); err != nil {
				return err
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			io.Copy(tarWriter, file)
		}
		return nil
	})

	if err != nil {
		log.Printf("Error writing tar: %s\n", err.Error())
		return "", err
	}

	err = tarWriter.Close()
	if err != nil {
		return "", err
	}

	err = tarFile.Close()
	if err != nil {
		return "", err
	}

	return tarFile.Name(), nil
}
