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

	"github.com/fsouza/go-dockerclient"
)

const (
	SVirtSandboxFileLabel = "system_u:object_r:svirt_sandbox_file_t:s0"
	ContainerInitDirName  = ".sti.init"
	ContainerInitDirPath  = "/" + ContainerInitDirName
)

// Request contains essential fields for any request: a Configuration, a base image, and an
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
	usage       bool
	workingDir  string
}

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	dockerClient *docker.Client
	request      *STIRequest
}

type STIResult struct {
	Success    bool
	Messages   []string
	WorkingDir string
	ImageID    string
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

	return &requestHandler{dockerClient, req}, nil
}

// Build processes a Request and returns a *Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
//
func Build(req *STIRequest) (result *STIResult, err error) {
	h, err := newHandler(req)
	if err != nil {
		return nil, err
	}

	h.request.workingDir, err = createWorkingDirectory()
	if err != nil {
		return nil, err
	}
	if h.request.PreserveWorkingDir {
		log.Printf("Temporary directory '%s' will be saved, not deleted\n", h.request.workingDir)
	} else {
		defer removeDirectory(h.request.workingDir, h.request.Verbose)
	}

	result = &STIResult{
		Success:    false,
		WorkingDir: h.request.workingDir,
	}

	dirs := []string{"tmp", "scripts", "defaultScripts"}
	for _, v := range dirs {
		err := os.Mkdir(filepath.Join(h.request.workingDir, v), 0700)
		if err != nil {
			return nil, err
		}
	}

	err = h.downloadScripts()
	if err != nil {
		return nil, err
	}

	if !h.request.usage {
		err = h.determineIncremental()
		if err != nil {
			return nil, err
		}
		if h.request.incremental {
			log.Printf("Existing image for tag %s detected for incremental build.\n", h.request.Tag)
		} else {
			log.Println("Clean build will be performed")
		}

		if h.request.Verbose {
			log.Printf("Performing source build from %s\n", h.request.Source)
		}

		if h.request.incremental {
			err = h.saveArtifacts()
			if err != nil {
				return nil, err
			}
		}
	}

	messages, imageID, err := h.buildInternal()
	result.Messages = messages
	result.ImageID = imageID
	if err == nil {
		result.Success = true
	}

	if h.request.CallbackUrl != "" {
		executeCallback(h.request.CallbackUrl, result)
	}

	return result, err
}

func (h requestHandler) buildInternal() (messages []string, imageID string, err error) {
	volumeMap := make(map[string]struct{})
	volumeMap["/tmp/src"] = struct{}{}
	volumeMap["/tmp/scripts"] = struct{}{}
	volumeMap["/tmp/defaultScripts"] = struct{}{}
	if h.request.incremental {
		volumeMap["/tmp/artifacts"] = struct{}{}
	}

	if h.request.Verbose {
		log.Printf("Using image name %s", h.request.BaseImage)
	}

	// get info about the specified image
	imageMetadata, err := h.checkAndPull(h.request.BaseImage)

	assemblePath := h.determineScriptPath("assemble")
	if assemblePath == "" {
		err = fmt.Errorf("No assemble script found in provided url, application source, or default image url. Aborting.")
		return
	}

	runPath := h.determineScriptPath("run")
	overrideRun := runPath != ""

	if h.request.Verbose {
		log.Printf("Using run script from %s", runPath)
		log.Printf("Using assemble script from %s", assemblePath)
	}

	user := ""
	if imageMetadata.Config != nil {
		user = imageMetadata.Config.User
	}

	hasUser := (user != "")
	if hasUser && h.request.Verbose {
		log.Printf("Image has username %s", user)
	}

	var cmd []string
	if hasUser {
		// run setup commands as root, then switch to container user
		// to execute the assemble script.
		cmd = []string{filepath.Join(ContainerInitDirPath, "init.sh")}
		volumeMap[ContainerInitDirPath] = struct{}{}
	} else if h.request.usage {
		// invoke assemble script with usage argument
		log.Println("Assemble script usage requested, invoking assemble script help")
		cmd = []string{"/bin/sh", "-c", "chmod 700 " + assemblePath + " && " + assemblePath + " -h"}
	} else {
		// normal assemble invocation
		cmd = []string{"/bin/sh", "-c", "chmod 700 " + assemblePath + " && " + assemblePath + " && mkdir -p /opt/sti/bin && cp " + runPath + " /opt/sti/bin && chmod 700 /opt/sti/bin/run"}
	}

	config := docker.Config{User: "root", Image: h.request.BaseImage, Cmd: cmd, Volumes: volumeMap}

	var cmdEnv []string
	if len(h.request.Environment) > 0 {
		for key, val := range h.request.Environment {
			cmdEnv = append(cmdEnv, key+"="+val)
		}
		config.Env = cmdEnv
	}
	if h.request.Verbose {
		log.Printf("Creating container using config: %+v\n", config)
	}

	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return
	}
	defer h.removeContainer(container.ID)

	binds := []string{filepath.Join(h.request.workingDir, "src") + ":/tmp/src"}
	binds = append(binds, filepath.Join(h.request.workingDir, "defaultScripts")+":/tmp/defaultScripts")
	binds = append(binds, filepath.Join(h.request.workingDir, "scripts")+":/tmp/scripts")
	if h.request.incremental {
		binds = append(binds, filepath.Join(h.request.workingDir, "artifacts")+":/tmp/artifacts")
	}

	if hasUser {
		containerInitDir := filepath.Join(h.request.workingDir, "tmp", ContainerInitDirName)
		err = os.MkdirAll(containerInitDir, 0700)
		if err != nil {
			return
		}

		err = chcon(SVirtSandboxFileLabel, containerInitDir, true)
		if err != nil {
			err = fmt.Errorf("unable to set SELinux context: %s", err.Error())
			return
		}

		buildScriptPath := filepath.Join(containerInitDir, "init.sh")
		var buildScript *os.File
		buildScript, err = os.OpenFile(buildScriptPath, os.O_CREATE|os.O_RDWR, 0700)
		if err != nil {
			return
		}

		templateFiller := struct {
			User         string
			Incremental  bool
			AssemblePath string
			RunPath      string
			Usage        bool
		}{user, h.request.incremental, assemblePath, runPath, h.request.Tag == ""}

		err = buildTemplate.Execute(buildScript, templateFiller)
		if err != nil {
			return
		}
		buildScript.Close()

		binds = append(binds, containerInitDir+":"+ContainerInitDirPath)
	}

	// only run chcon if it's not an incremental build, as saveArtifacts will have
	// already run chcon if it is incremental
	if !h.request.incremental {
		err = chcon(SVirtSandboxFileLabel, h.request.workingDir, true)
		if err != nil {
			err = fmt.Errorf("Unable to set SELinux context for %s: %s", h.request.workingDir, err.Error())
			return
		}
	}

	hostConfig := docker.HostConfig{Binds: binds}
	if h.request.Verbose {
		log.Printf("Starting container with config: %+v\n", hostConfig)
	}

	err = h.dockerClient.StartContainer(container.ID, &hostConfig)
	if err != nil {
		return
	}

	attachOpts := docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stdout,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
		Logs:         true}

	err = h.dockerClient.AttachToContainer(attachOpts)
	if err != nil {
		log.Println("Couldn't attach to container")
	}

	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return
	}

	if exitCode != 0 {
		err = ErrBuildFailed
		return
	}

	if h.request.usage {
		// this was just a request for assemble usage, so return without committing
		// a new runnable image.
		return
	}

	config = docker.Config{Image: h.request.BaseImage, Env: cmdEnv}
	if overrideRun {
		config.Cmd = []string{"/opt/sti/bin/run"}
	} else {
		config.Cmd = imageMetadata.Config.Cmd
		config.Entrypoint = imageMetadata.Config.Entrypoint
	}
	if hasUser {
		config.User = user
	}

	previousImageId := ""
	if h.request.incremental && h.request.RemovePreviousImage {
		imageMetadata, err := h.dockerClient.InspectImage(h.request.Tag)
		if err == nil {
			previousImageId = imageMetadata.ID
		} else {
			log.Printf("Error retrieving previous image's metadata: %s\n", err.Error())
		}
	}

	if h.request.Verbose {
		log.Printf("Commiting container with config: %+v\n", config)
	}

	builtImage, err := h.dockerClient.CommitContainer(docker.CommitContainerOptions{Container: container.ID, Repository: h.request.Tag, Run: &config})
	if err != nil {
		err = ErrBuildFailed
		return
	}

	if h.request.Verbose {
		log.Printf("Built image: %+v\n", builtImage)
	}

	if h.request.incremental && h.request.RemovePreviousImage && previousImageId != "" {
		log.Printf("Removing previously-tagged image %s\n", previousImageId)
		err = h.dockerClient.RemoveImage(previousImageId)
		if err != nil {
			log.Printf("Unable to remove previous image: %s\n", err.Error())
		}
	}

	return
}

func (h requestHandler) downloadScripts() error {
	var wg sync.WaitGroup

	downloadAsync := func(scriptUrl *url.URL, targetFile string) {
		defer wg.Done()
		err := downloadFile(scriptUrl, targetFile, h.request.Verbose)
		if err != nil {
			log.Printf("Failed to download '%s' (%s)\n", scriptUrl, err.Error())
		}
	}

	if h.request.ScriptsUrl != "" {
		for file, url := range h.prepareScriptDownload(h.request.workingDir+"/scripts", h.request.ScriptsUrl) {
			wg.Add(1)
			go downloadAsync(url, file)
		}
	}

	defaultUrl, err := h.getDefaultUrl()
	if err != nil {
		return fmt.Errorf("Unable to retrieve the default STI scripts URL: %s", err.Error())
	}

	if defaultUrl != "" {
		for file, url := range h.prepareScriptDownload(h.request.workingDir+"/defaultScripts", defaultUrl) {
			wg.Add(1)
			go downloadAsync(url, file)
		}
	}

	targetSourceDir := filepath.Join(h.request.workingDir, "src")
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = h.prepareSourceDir(h.request.Source, targetSourceDir, h.request.Ref)
		if err != nil {
			fmt.Printf("ERROR: Unable to fetch the application source.\n")
		}
	}()

	// Wait for the scripts and the source code download to finish.
	//
	wg.Wait()

	return nil
}

func (h requestHandler) determineIncremental() error {
	var err error
	incremental := !h.request.Clean

	if incremental {
		// can only do incremental build if runtime image exists
		incremental, err = h.isImageInLocalRegistry(h.request.Tag)
		if err != nil {
			return err
		}
	}
	if incremental {
		// check if a save-artifacts script exists in anything provided to the build
		// without it, we cannot do incremental builds
		incremental = h.determineScriptPath("save-artifacts") != ""
	}

	h.request.incremental = incremental

	return nil
}

func (h requestHandler) getDefaultUrl() (string, error) {
	image := h.request.BaseImage
	imageMetadata, err := h.checkAndPull(image)
	if err != nil {
		return "", err
	}
	var defaultScriptsUrl string
	env := imageMetadata.ContainerConfig.Env
	for _, v := range env {
		if strings.HasPrefix(v, "STI_SCRIPTS_URL=") {
			t := strings.Split(v, "=")
			defaultScriptsUrl = t[1]
			break
		}
	}
	if h.request.Verbose {
		log.Printf("Image contains default script url %s", defaultScriptsUrl)
	}
	return defaultScriptsUrl, nil
}

func (h requestHandler) determineScriptPath(script string) string {
	contextDir := h.request.workingDir

	if _, err := os.Stat(filepath.Join(contextDir, "scripts", script)); err == nil {
		// if the invoker provided a script via a url, prefer that.
		if h.request.Verbose {
			log.Printf("Using %s script from user provided url", script)
		}
		return filepath.Join("/tmp", "scripts", script)
	} else if _, err := os.Stat(filepath.Join(contextDir, "src", ".sti", "bin", script)); err == nil {
		// if they provided one in the app source, that is preferred next
		if h.request.Verbose {
			log.Printf("Using %s script from application source", script)
		}
		return filepath.Join("/tmp", "src", ".sti", "bin", script)
	} else if _, err := os.Stat(filepath.Join(contextDir, "defaultScripts", script)); err == nil {
		// lowest priority: script provided by default url reference in the image.
		if h.request.Verbose {
			log.Printf("Using %s script from image default url", script)
		}
		return filepath.Join("/tmp", "defaultScripts", script)
	}
	return ""
}

// Turn the script name into proper URL
func (h requestHandler) prepareScriptDownload(targetDir, baseUrl string) map[string]*url.URL {

	os.MkdirAll(targetDir, 0700)

	files := []string{"save-artifacts", "assemble", "run"}
	urls := make(map[string]*url.URL)

	for _, file := range files {
		url, err := url.Parse(baseUrl + "/" + file)
		if err != nil {
			log.Printf("[WARN] Unable to parse script URL: %n\n", baseUrl+"/"+file)
			continue
		}

		urls[targetDir+"/"+file] = url
	}

	return urls
}

func (h requestHandler) saveArtifacts() error {
	artifactTmpDir := filepath.Join(h.request.workingDir, "artifacts")
	err := os.Mkdir(artifactTmpDir, 0700)
	if err != nil {
		return err
	}

	image := h.request.Tag

	if h.request.Verbose {
		log.Printf("Saving build artifacts from image %s to path %s\n", image, artifactTmpDir)
	}

	imageMetadata, err := h.dockerClient.InspectImage(image)
	if err != nil {
		return err
	}

	saveArtifactsScriptPath := h.determineScriptPath("save-artifacts")

	user := imageMetadata.Config.User
	hasUser := (user != "")
	if h.request.Verbose {
		log.Printf("Artifact image hasUser=%t, user is %s\n", hasUser, user)
	}

	volumeMap := make(map[string]struct{})
	volumeMap["/tmp/artifacts"] = struct{}{}
	volumeMap["/tmp/src"] = struct{}{}
	volumeMap["/tmp/scripts"] = struct{}{}
	volumeMap["/tmp/defaultScripts"] = struct{}{}

	cmd := []string{"/bin/sh", "-c", "chmod 777 " + saveArtifactsScriptPath + " && " + saveArtifactsScriptPath}

	if hasUser {
		volumeMap[ContainerInitDirPath] = struct{}{}
		cmd = []string{filepath.Join(ContainerInitDirPath, "init.sh")}
	}

	config := docker.Config{User: "root", Image: image, Cmd: cmd, Volumes: volumeMap}
	if h.request.Verbose {
		log.Printf("Creating container using config: %+v\n", config)
	}
	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return err
	}
	defer h.removeContainer(container.ID)

	binds := []string{artifactTmpDir + ":/tmp/artifacts"}
	binds = append(binds, filepath.Join(h.request.workingDir, "src")+":/tmp/src")
	binds = append(binds, filepath.Join(h.request.workingDir, "defaultScripts")+":/tmp/defaultScripts")
	binds = append(binds, filepath.Join(h.request.workingDir, "scripts")+":/tmp/scripts")

	if hasUser {
		// TODO: add custom errors?
		if h.request.Verbose {
			log.Println("Creating stub file")
		}
		stubFile, err := os.OpenFile(filepath.Join(artifactTmpDir, ".stub"), os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			return err
		}
		defer stubFile.Close()

		containerInitDir := filepath.Join(h.request.workingDir, "tmp", ContainerInitDirName)
		if h.request.Verbose {
			log.Printf("Creating dir %+v\n", containerInitDir)
		}
		err = os.MkdirAll(containerInitDir, 0700)
		if err != nil {
			return err
		}

		err = chcon(SVirtSandboxFileLabel, containerInitDir, true)
		if err != nil {
			return fmt.Errorf("unable to set SELinux context: %s", err.Error())
		}

		initScriptPath := filepath.Join(containerInitDir, "init.sh")
		if h.request.Verbose {
			log.Printf("Writing %+v\n", initScriptPath)
		}
		initScript, err := os.OpenFile(initScriptPath, os.O_CREATE|os.O_RDWR, 0766)
		if err != nil {
			return err
		}

		err = saveArtifactsInitTemplate.Execute(initScript, struct {
			User              string
			SaveArtifactsPath string
		}{user, saveArtifactsScriptPath})
		if err != nil {
			return err
		}
		initScript.Close()

		binds = append(binds, containerInitDir+":"+ContainerInitDirPath)
	}

	err = chcon(SVirtSandboxFileLabel, h.request.workingDir, true)
	if err != nil {
		err = fmt.Errorf("Unable to set SELinux context for %s: %s", h.request.workingDir, err.Error())
		return err
	}

	hostConfig := docker.HostConfig{Binds: binds}
	if h.request.Verbose {
		log.Printf("Starting container with host config %+v\n", hostConfig)
	}
	err = h.dockerClient.StartContainer(container.ID, &hostConfig)
	if err != nil {
		return err
	}

	attachOpts := docker.AttachToContainerOptions{Container: container.ID, OutputStream: os.Stdout,
		ErrorStream: os.Stdout, Stream: true, Stdout: true, Stderr: true, Logs: true}
	err = h.dockerClient.AttachToContainer(attachOpts)
	if err != nil {
		log.Printf("Couldn't attach to container")
	}

	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		if h.request.Verbose {
			log.Printf("Exit code: %d", exitCode)
		}
		return ErrSaveArtifactsFailed
	}

	return nil
}

func (h requestHandler) prepareSourceDir(source, targetSourceDir, ref string) error {
	if validCloneSpec(source, h.request.Verbose) {
		log.Printf("Downloading %s to directory %s\n", source, targetSourceDir)
		err := gitClone(source, targetSourceDir)
		if err != nil {
			if h.request.Verbose {
				log.Printf("Git clone failed: %+v", err)
			}

			return err
		}

		if ref != "" {
			if h.request.Verbose {
				log.Printf("Checking out ref %s", ref)
			}

			err := gitCheckout(targetSourceDir, ref, h.request.Verbose)
			if err != nil {
				return err
			}
		}
	} else {
		// TODO: investigate using bind-mounts instead
		copy(source, targetSourceDir)
	}

	return nil
}
