package sti

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// SchemeReaders create an io.Reader from the given url.
//
type SchemeReader func(*url.URL) (io.ReadCloser, error)

var schemeReaders = map[string]SchemeReader{
	"http":  readerFromHttpUrl,
	"https": readerFromHttpUrl,
	"file":  readerFromFileUrl,
}

// This SchemeReader can produce an io.Reader from a file URL.
//
func readerFromHttpUrl(url *url.URL) (io.ReadCloser, error) {
	resp, err := http.Get(url.String())
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
		}
		return nil, err
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return resp.Body, nil
	} else {
		return nil, fmt.Errorf("Failed to retrieve %s, response code %d", url.String(), resp.StatusCode)
	}
}

func readerFromFileUrl(url *url.URL) (io.ReadCloser, error) {
	return os.Open(url.Path)
}

// Determine whether a file exists in a container.
func FileExistsInContainer(dockerClient *docker.Client, cId string, path string) bool {
	var buf []byte
	writer := bytes.NewBuffer(buf)

	err := dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{writer, cId, path})
	content := writer.String()

	return ((err == nil) && ("" != content))
}

func stringInSlice(s string, slice []string) bool {
	for _, element := range slice {
		if s == element {
			return true
		}
	}

	return false
}

func writeTar(tw *tar.Writer, path string, relative string, fi os.FileInfo) error {
	fr, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fr.Close()

	h := new(tar.Header)
	h.Name = strings.Replace(path, relative, ".", 1)
	h.Size = fi.Size()
	h.Mode = int64(fi.Mode())
	h.ModTime = fi.ModTime()

	err = tw.WriteHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, fr)
	return err
}

func tarDirectory(dir string) (*os.File, error) {
	fw, err := ioutil.TempFile("", "sti-tar")
	if err != nil {
		return nil, err
	}
	defer fw.Close()

	tw := tar.NewWriter(fw)
	defer tw.Close()

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			err = writeTar(tw, path, dir, info)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return fw, nil
}

func downloadFile(url *url.URL, targetFile string, verbose bool) error {
	sr := schemeReaders[url.Scheme]

	reader, err := sr(url)

	defer reader.Close()

	if err != nil {
		log.Printf("ERROR: Reading error while downloading %s (%s)\n", url.String(), err)
		return err
	}

	out, err := os.Create(targetFile)
	defer out.Close()

	if err != nil {
		defer os.Remove(targetFile)
		log.Printf("ERROR: Unable to create target file %s (%s)\n", targetFile, err)
		return err
	}

	_, err = io.Copy(out, reader)

	if err != nil {
		defer os.Remove(targetFile)
		log.Printf("Skipping file %s due to error copying from source: %s\n", targetFile, err)
	}

	if verbose {
		log.Printf("Downloaded '%s'\n", url.String())
	}
	return nil
}

func copy(sourcePath string, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		err = os.Mkdir(targetPath, 0700)
		if err != nil {
			return err
		}

		targetPath = filepath.Join(targetPath, filepath.Base(sourcePath))
	}

	cmd := exec.Command("cp", "-ad", sourcePath, targetPath)
	return cmd.Run()
}

func imageHasEntryPoint(image *docker.Image) bool {
	found := (image.ContainerConfig.Entrypoint != nil)

	if !found && image.Config != nil {
		found = image.Config.Entrypoint != nil
	}

	return found
}

func executeCallback(callbackUrl string, result *STIResult) {
	buf := new(bytes.Buffer)
	writer := bufio.NewWriter(buf)
	for _, message := range result.Messages {
		fmt.Fprintln(writer, message)
	}
	writer.Flush()

	d := map[string]interface{}{
		"payload": buf.String(),
		"success": result.Success,
	}

	jsonBuffer := new(bytes.Buffer)
	writer = bufio.NewWriter(jsonBuffer)
	jsonWriter := json.NewEncoder(writer)
	jsonWriter.Encode(d)
	writer.Flush()

	var resp *http.Response
	var err error

	for retries := 0; retries < 3; retries++ {
		resp, err = http.Post(callbackUrl, "application/json", jsonBuffer)
		if err != nil {
			errorMessage := fmt.Sprintf("Unable to invoke callback: %s", err.Error())
			result.Messages = append(result.Messages, errorMessage)
		}
		if resp != nil {
			if resp.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("Callback returned with error code: %d", resp.StatusCode)
				result.Messages = append(result.Messages, errorMessage)
			}
			break
		}
	}
}

func removeDirectory(dir string, verbose bool) {
	if verbose {
		log.Printf("Removing directory '%s'\n", dir)
	}

	err := os.RemoveAll(dir)
	if err != nil {
		log.Printf("Error removing directory '%s': %s\n", dir, err.Error())
	}
}

func createWorkingDirectory() (directory string, err error) {
	directory, err = ioutil.TempDir("", "sti")
	if err != nil {
		return "", fmt.Errorf("Error creating temporary directory '%s': %s\n", directory, err.Error())
	}

	return directory, err
}
