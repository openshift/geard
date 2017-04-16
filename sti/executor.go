package sti

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fsouza/go-dockerclient"
)

// STIRequest contains essential fields for any request: a Configuration, a base image, and an
// optional runtime image.
type STIRequest struct {
	BaseImage           string
	DockerSocket        string
	DockerTimeout       int
	Verbose             bool
	PreserveWorkingDir  bool
	Source              string
	Ref                 string
	Tag                 string
	Clean               bool
	RemovePreviousImage bool
	Environment         map[string]string
	Writer              io.Writer
	CallbackUrl         string
	ScriptsUrl          string

	incremental bool
	workingDir  string
}

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	dockerClient *docker.Client
	request      *STIRequest
	result       *STIResult
	postExecutor postExecutor
}

type STIResult struct {
	Success    bool
	Messages   []string
	WorkingDir string
	ImageID    string
}

type postExecutor interface {
	postExecute(container *docker.Container) error
}

// Returns a new handler for a given request.
func newHandler(req *STIRequest) (*requestHandler, error) {
	if req.Verbose {
		log.Printf("Using docker socket: %s\n", req.DockerSocket)
	}

	dockerClient, err := docker.NewClient(req.DockerSocket)
	if err != nil {
		return nil, ErrDockerConnectionFailed
	}

	return &requestHandler{dockerClient, req, nil, nil}, nil
}

func (h *requestHandler) setup(requiredScripts, optionalScripts []string, command string) error {
	var err error
	h.request.workingDir, err = createWorkingDirectory()
	if err != nil {
		return err
	}

	h.result = &STIResult{
		Success:    false,
		WorkingDir: h.request.workingDir,
	}

	dirs := []string{"upload/scripts", "downloads/scripts", "downloads/defaultScripts"}
	for _, v := range dirs {
		err := os.MkdirAll(filepath.Join(h.request.workingDir, v), 0700)
		if err != nil {
			return err
		}
	}

	err = h.downloadAndInstallScripts(requiredScripts, true)
	if err != nil {
		return err
	}

	err = h.downloadAndInstallScripts(optionalScripts, false)
	if err != nil {
		return err
	}

	return nil
}

func (h *requestHandler) downloadAndInstallScripts(scripts []string, required bool) error {
	err := h.downloadScripts(scripts)
	if err != nil {
		return err
	}

	for _, script := range scripts {
		scriptPath := h.determineScriptPath(script)
		if required && scriptPath == "" {
			err = fmt.Errorf("No %s script found in provided url, application source, or default image url. Aborting.", script)
			return err
		}
		err = h.installScript(scriptPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *requestHandler) downloadScripts(scripts []string) error {
	if len(scripts) == 0 {
		return nil
	}

	var (
		wg            sync.WaitGroup
		errorCount    int32 = 0
		downloadCount int32 = 0
	)

	downloadAsync := func(scriptUrl *url.URL, targetFile string) {
		defer wg.Done()
		err := downloadFile(scriptUrl, targetFile, h.request.Verbose)
		if err != nil {
			return
		}
		atomic.AddInt32(&downloadCount, 1)

		err = os.Chmod(targetFile, 0700)
		if err != nil {
			atomic.AddInt32(&errorCount, 1)
		}
	}

	if h.request.ScriptsUrl != "" {
		destDir := filepath.Join(h.request.workingDir, "/downloads/scripts")
		for file, url := range h.prepareScriptDownload(scripts, destDir, h.request.ScriptsUrl) {
			wg.Add(1)
			go downloadAsync(url, file)
		}
	}

	defaultUrl, err := h.getDefaultUrl()
	if err != nil {
		return fmt.Errorf("Unable to retrieve the default STI scripts URL: %s", err.Error())
	}

	if defaultUrl != "" {
		destDir := filepath.Join(h.request.workingDir, "/downloads/defaultScripts")
		for file, url := range h.prepareScriptDownload(scripts, destDir, defaultUrl) {
			wg.Add(1)
			go downloadAsync(url, file)
		}
	}

	// Wait for the script downloads to finish.
	//
	wg.Wait()
	if downloadCount == 0 || errorCount > 0 {
		return ErrScriptsDownloadFailed
	}

	return nil
}

func (h *requestHandler) getDefaultUrl() (string, error) {
	image := h.request.BaseImage
	imageMetadata, err := h.checkAndPull(image)
	if err != nil {
		return "", err
	}
	var defaultScriptsUrl string
	env := append(imageMetadata.ContainerConfig.Env, imageMetadata.Config.Env...)
	for _, v := range env {
		if strings.HasPrefix(v, "STI_SCRIPTS_URL=") {
			t := strings.Split(v, "=")
			defaultScriptsUrl = t[1]
			break
		}
	}
	if h.request.Verbose {
		log.Printf("Image contains default script url '%s'", defaultScriptsUrl)
	}
	return defaultScriptsUrl, nil
}

func (h *requestHandler) determineScriptPath(script string) string {
	locations := []string{
		"downloads/scripts",
		"upload/src/.sti/bin",
		"downloads/defaultScripts",
	}
	descriptions := []string{
		"user provided url",
		"application source",
		"default url reference in the image",
	}

	for i, location := range locations {
		path := filepath.Join(h.request.workingDir, location, script)
		if h.request.Verbose {
			log.Printf("Looking for %s script at %s", script, path)
		}
		if _, err := os.Stat(path); err == nil {
			if h.request.Verbose {
				log.Printf("Found %s script from %s.", script, descriptions[i])
			}
			return path
		}
	}

	return ""
}

func (h *requestHandler) installScript(path string) error {
	script := filepath.Base(path)
	return os.Rename(path, filepath.Join(h.request.workingDir, "upload/scripts", script))
}

// Turn the script name into proper URL
func (h *requestHandler) prepareScriptDownload(scripts []string, targetDir, baseUrl string) map[string]*url.URL {

	os.MkdirAll(targetDir, 0700)

	urls := make(map[string]*url.URL)

	for _, script := range scripts {
		url, err := url.Parse(baseUrl + "/" + script)
		if err != nil {
			log.Printf("[WARN] Unable to parse script URL: %n\n", baseUrl+"/"+script)
			continue
		}

		urls[targetDir+"/"+script] = url
	}

	return urls
}

func (h *requestHandler) generateConfigEnv() []string {
	var configEnv []string
	if len(h.request.Environment) > 0 {
		for key, val := range h.request.Environment {
			configEnv = append(configEnv, key+"="+val)
		}
	}
	return configEnv
}

func (h *requestHandler) execute(command string) error {
	if h.request.Verbose {
		log.Printf("Using image name %s", h.request.BaseImage)
	}

	// get info about the specified image
	imageMetadata, err := h.checkAndPull(h.request.BaseImage)

	cmd := imageMetadata.Config.Cmd
	cmd = append(cmd, command)
	config := docker.Config{
		Image:     h.request.BaseImage,
		OpenStdin: true,
		StdinOnce: true,
		Cmd:       cmd,
		Env:       h.generateConfigEnv(),
	}

	if h.request.Verbose {
		log.Printf("Creating container using config: %+v\n", config)
	}

	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return err
	}
	defer h.removeContainer(container.ID)

	tarFileName, err := h.createTarUpload()
	if err != nil {
		return err
	}

	tarFile, err := os.Open(tarFileName)
	if err != nil {
		return err
	}

	if h.request.Verbose {
		log.Printf("Attaching to container")
	}
	attached := make(chan struct{})
	attachOpts := docker.AttachToContainerOptions{
		Container:    container.ID,
		InputStream:  tarFile,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stdout,
		Stream:       true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Logs:         true,
		Success:      attached,
	}
	go h.dockerClient.AttachToContainer(attachOpts)
	attached <- <-attached

	if h.request.Verbose {
		log.Printf("Starting container")
	}
	err = h.dockerClient.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}

	if h.request.Verbose {
		log.Printf("Waiting for container")
	}
	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return ErrBuildFailed
	}

	if h.postExecutor != nil {
		if h.request.Verbose {
			log.Printf("Invoking postExecution function")
		}
		err = h.postExecutor.postExecute(container)
		if err != nil {
			return err
		}
	}

	//TODO The code in master right now never populates these messages -
	//     what should be going in here?
	//h.result.Messages = messages
	if err == nil {
		h.result.Success = true
	}

	return nil
}

func (h *requestHandler) cleanup() {
	if h.request.PreserveWorkingDir {
		log.Printf("Temporary directory '%s' will be saved, not deleted\n", h.request.workingDir)
	} else {
		removeDirectory(h.request.workingDir, h.request.Verbose)
	}
}
