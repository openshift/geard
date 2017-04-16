package sti

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// createTarUpload creates a tar file that contains the contents of the upload
// directory and returns the name of the tar file
func (h *requestHandler) createTarUpload() (string, error) {
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

			header.Name = path[1+len(uploadDir):]

			if h.request.Verbose {
				log.Printf("Adding to tar: %s as %s\n", path, header.Name)
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

func (h *requestHandler) extractTarStream(artifactTmpDir string, reader io.Reader) error {
	tarReader := tar.NewReader(reader)
	errorChannel := make(chan error)
	timeout := 5 * time.Second
	timeoutTimer := time.NewTimer(timeout)
	go func() {
		for {
			header, err := tarReader.Next()
			timeoutTimer.Reset(timeout)
			if err == io.EOF {
				errorChannel <- nil
				break
			}
			if err != nil {
				log.Printf("Error reading next tar header: %s", err.Error())
				errorChannel <- err
				break
			}
			if header.FileInfo().IsDir() {
				dirPath := filepath.Join(artifactTmpDir, header.Name)
				err = os.MkdirAll(dirPath, 0700)
				if err != nil {
					log.Printf("Error creating dir %s: %s", dirPath, err.Error())
					errorChannel <- err
					break
				}
			} else {
				dir := filepath.Dir(header.Name)
				dirPath := filepath.Join(artifactTmpDir, dir)
				err = os.MkdirAll(dirPath, 0700)
				if err != nil {
					log.Printf("Error creating dir %s: %s", dirPath, err.Error())
					errorChannel <- err
					break
				}
				//TODO should this be OpenFile so we can set the perms to 600 or 660?
				path := filepath.Join(artifactTmpDir, header.Name)
				if h.request.Verbose {
					log.Printf("Creating %s", path)
				}
				file, err := os.Create(path)
				if err != nil {
					log.Printf("Error creating file %s: %s", path, err.Error())
					errorChannel <- err
					break
				}
				defer file.Close()

				if h.request.Verbose {
					log.Printf("Extracting/writing %s", path)
				}
				written, err := io.Copy(file, tarReader)
				if err != nil {
					log.Printf("Error writing file: %s", err.Error())
					errorChannel <- err
					break
				}
				if written != header.Size {
					message := fmt.Sprintf("Wrote %d bytes, expected to write %d\n", written, header.Size)
					log.Println(message)
					errorChannel <- fmt.Errorf(message)
					break
				}
				if h.request.Verbose {
					log.Printf("Done with %s", path)
				}
			}
		}
	}()

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				log.Printf("Error reading tar stream")
			}
			if h.request.Verbose {
				log.Printf("Done reading tar stream")
			}
			return err
		case <-timeoutTimer.C:
			return fmt.Errorf("Timeout waiting for artifacts tar stream")
		}
	}
}
