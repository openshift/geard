package sti

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/fsouza/go-dockerclient"
)

type BuildRequest struct {
	Request
	Source      string
	Tag         string
	Clean       bool
	Environment map[string]string
	Method      string
	Writer      io.Writer
}

type BuildResult STIResult

// Build processes a BuildRequest and returns a *BuildResult and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func Build(req BuildRequest) (*BuildResult, error) {
	method := req.Method
	if method == "" {
		req.Method = "build"
	} else {
		if !stringInSlice(method, []string{"run", "build"}) {
			return nil, ErrInvalidBuildMethod
		}
	}

	h, err := newHandler(req.Request)
	if err != nil {
		return nil, err
	}

	incremental := !req.Clean

	// If a runtime image is defined, check for the presence of an
	// existing build image for the app to determine if an incremental
	// build should be performed
	tag := req.Tag
	if req.RuntimeImage != "" {
		tag = tag + "-build"
	}

	if incremental {
		exists, err := h.isImageInLocalRegistry(tag)

		if err != nil {
			return nil, err
		}

		if exists {
			incremental, err = h.detectIncrementalBuild(tag)
			if err != nil {
				return nil, err
			}
		} else {
			incremental = false
		}
	}

	if h.debug {
		if incremental {
			log.Printf("Existing image for tag %s detected for incremental build\n", tag)
		} else {
			log.Printf("Clean build will be performed")
		}
	}

	var result *BuildResult

	if req.RuntimeImage == "" {
		result, err = h.build(req, incremental)
	} else {
		result, err = h.extendedBuild(req, incremental)
	}

	return result, err
}

func (h requestHandler) detectIncrementalBuild(tag string) (bool, error) {
	if h.debug {
		log.Printf("Determining whether image %s is compatible with incremental build", tag)
	}

	container, err := h.containerFromImage(tag)
	if err != nil {
		return false, err
	}
	defer h.removeContainer(container.ID)

	return FileExistsInContainer(h.dockerClient, container.ID, "/usr/bin/save-artifacts"), nil
}

func (h requestHandler) build(req BuildRequest, incremental bool) (*BuildResult, error) {
	if h.debug {
		log.Printf("Performing source build from %s\n", req.Source)
	}
	if incremental {
		artifactTmpDir := filepath.Join(req.WorkingDir, "artifacts")
		err := os.Mkdir(artifactTmpDir, 0700)
		if err != nil {
			return nil, err
		}

		err = h.saveArtifacts(req.Tag, artifactTmpDir)
		if err != nil {
			return nil, err
		}
	}

	targetSourceDir := filepath.Join(req.WorkingDir, "src")
	err := h.prepareSourceDir(req.Source, targetSourceDir)
	if err != nil {
		return nil, err
	}

	return h.buildDeployableImage(req, req.BaseImage, req.WorkingDir, incremental)
}

func (h requestHandler) extendedBuild(req BuildRequest, incremental bool) (*BuildResult, error) {
	var (
		buildImageTag = req.Tag + "-build"
		wd            = req.WorkingDir

		builderBuildDir     = filepath.Join(wd, "build")
		previousBuildVolume = filepath.Join(builderBuildDir, "last_build_artifacts")
		inputSourceDir      = filepath.Join(builderBuildDir, "src")

		runtimeBuildDir = filepath.Join(wd, "runtime")
		outputSourceDir = filepath.Join(runtimeBuildDir, "src")
	)

	for _, dir := range []string{builderBuildDir, runtimeBuildDir, previousBuildVolume, outputSourceDir} {
		err := os.Mkdir(dir, 0700)
		if err != nil {
			return nil, err
		}
	}

	if incremental {
		err := h.saveArtifacts(buildImageTag, previousBuildVolume)
		if err != nil {
			return nil, err
		}
	}

	err := h.prepareSourceDir(req.Source, inputSourceDir)
	if err != nil {
		return nil, err
	}

	// TODO: necessary to specify these, if specifying bind-mounts?
	volumeMap := make(map[string]struct{})
	volumeMap["/usr/artifacts"] = struct{}{}
	volumeMap["/usr/src"] = struct{}{}
	volumeMap["/usr/build"] = struct{}{}

	bindMounts := []string{
		previousBuildVolume + ":/usr/artifacts",
		inputSourceDir + ":/usr/src",
		outputSourceDir + ":/usr/build",
	}

	if h.debug {
		log.Println("Creating build container to run source build")
	}

	config := docker.Config{Image: req.BaseImage, Cmd: []string{"/usr/bin/prepare"}, Volumes: volumeMap}
	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return nil, err
	}
	cID := container.ID

	if h.debug {
		log.Printf("Build container: %s\n", cID)
	} else {
		defer h.removeContainer(cID)
	}

	hostConfig := docker.HostConfig{Binds: bindMounts}
	err = h.dockerClient.StartContainer(cID, &hostConfig)
	if err != nil {
		return nil, err
	}

	exitCode, err := h.dockerClient.WaitContainer(cID)
	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		return nil, ErrBuildFailed
	}

	buildResult, err := h.buildDeployableImage(req, req.RuntimeImage, runtimeBuildDir, false)
	if err != nil {
		return nil, err
	}

	if h.debug {
		log.Printf("Commiting build container %s to tag %s", cID, buildImageTag)
	}

	err = h.commitContainer(cID, buildImageTag)
	if err != nil {
		log.Printf("Unable commit container %s to tag %s\n", cID, buildImageTag)
	}

	return buildResult, nil
}

func (h requestHandler) saveArtifacts(image string, path string) error {
	if h.debug {
		log.Printf("Saving build artifacts from image %s to path %s\n", image, path)
	}

	volumeMap := make(map[string]struct{})
	volumeMap["/usr/artifacts"] = struct{}{}

	config := docker.Config{Image: image, Cmd: []string{"/usr/bin/save-artifacts"}, Volumes: volumeMap}
	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return err
	}
	defer h.removeContainer(container.ID)

	hostConfig := docker.HostConfig{Binds: []string{path + ":/usr/artifacts"}}
	err = h.dockerClient.StartContainer(container.ID, &hostConfig)
	if err != nil {
		return err
	}

	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return ErrSaveArtifactsFailed
	}

	return nil
}

func (h requestHandler) prepareSourceDir(source string, targetSourceDir string) error {
	re := regexp.MustCompile("^git://")

	if re.MatchString(source) {
		if h.debug {
			log.Printf("Fetching %s to directory %s", source, targetSourceDir)
		}
		err := gitClone(source, targetSourceDir)
		if err != nil {
			if h.debug {
				log.Printf("Git clone failed: %+v", err)
			}
			return err
		}
	} else {
		// TODO: investigate using bind-mounts instead
		copy(source, targetSourceDir)
	}

	return nil
}

var dockerFileTemplate = template.Must(template.New("Dockerfile").Parse("" +
	"FROM {{.BaseImage}}\n" +
	"ADD ./src /usr/src/\n" +
	"{{if .Incremental}}ADD ./artifacts /usr/artifacts\n{{end}}" +
	"{{range $key, $value := .Environment}}ENV {{$key}} {{$value}}\n{{end}}" +
	"RUN /usr/bin/prepare\n" +
	"CMD /usr/bin/run\n"))

func (h requestHandler) buildDeployableImage(req BuildRequest, image string, contextDir string, incremental bool) (*BuildResult, error) {
	if req.Method == "run" {
		return h.buildDeployableImageWithDockerRun(req, image, contextDir, incremental)
	}

	return h.buildDeployableImageWithDockerBuild(req, image, contextDir, incremental)
}

func (h requestHandler) buildDeployableImageWithDockerBuild(req BuildRequest, image string, contextDir string, incremental bool) (*BuildResult, error) {
	dockerFilePath := filepath.Join(contextDir, "Dockerfile")
	dockerFile, err := openFileExclusive(dockerFilePath, 0700)
	if err != nil {
		return nil, err
	}
	defer dockerFile.Close()

	templateFiller := struct {
		BaseImage   string
		Environment map[string]string
		Incremental bool
	}{image, req.Environment, incremental}
	err = dockerFileTemplate.Execute(dockerFile, templateFiller)
	if err != nil {
		return nil, ErrCreateDockerfileFailed
	}

	if h.debug {
		log.Printf("Wrote Dockerfile for build to %s\n", dockerFilePath)
	}

	tarBall, err := tarDirectory(contextDir)
	if err != nil {
		return nil, err
	}

	if h.debug {
		log.Printf("Created tarball for %s at %s\n", contextDir, tarBall.Name())
	}

	tarInput, err := os.Open(tarBall.Name())
	if err != nil {
		return nil, err
	}
	defer tarInput.Close()
	tarReader := bufio.NewReader(tarInput)
	var output []string

	if req.Writer != nil {
		err = h.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, req.Writer, ""})
	} else {
		var buf []byte
		writer := bytes.NewBuffer(buf)
		err = h.dockerClient.BuildImage(docker.BuildImageOptions{req.Tag, false, false, true, tarReader, writer, ""})
		rawOutput := writer.String()
		output = strings.Split(rawOutput, "\n")
	}

	if err != nil {
		return nil, err
	}

	return &BuildResult{true, output}, nil
}

func (h requestHandler) buildDeployableImageWithDockerRun(req BuildRequest, image string, contextDir string, incremental bool) (*BuildResult, error) {
	volumeMap := make(map[string]struct{})
	volumeMap["/usr/src"] = struct{}{}
	if incremental {
		volumeMap["/usr/artifacts"] = struct{}{}
	}

	config := docker.Config{Image: image, Cmd: []string{"/usr/bin/prepare"}, Volumes: volumeMap}
	var cmdEnv []string
	if len(req.Environment) > 0 {
		for key, val := range req.Environment {
			cmdEnv = append(cmdEnv, key+"="+val)
		}
		config.Env = cmdEnv
	}
	if h.debug {
		log.Printf("Creating container using config: %+v\n", config)
	}

	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return nil, err
	}
	defer h.removeContainer(container.ID)

	binds := []string{
		filepath.Join(contextDir, "src") + ":/usr/src",
	}
	if incremental {
		binds = append(binds, filepath.Join(contextDir, "artifacts")+":/usr/artifacts")
	}

	hostConfig := docker.HostConfig{Binds: binds}
	if h.debug {
		log.Printf("Starting container with config: %+v\n", hostConfig)
	}

	err = h.dockerClient.StartContainer(container.ID, &hostConfig)
	if err != nil {
		return nil, err
	}

	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		return nil, ErrBuildFailed
	}

	// config = docker.Config{Image: image, Cmd: []string{"/usr/bin/run"}, Env: cmdEnv}
	// if h.debug {
	// 	log.Printf("Commiting container with config: %+v\n", config)
	// }

	// builtImage, err := h.dockerClient.CommitContainer(docker.CommitContainerOptions{Container: container.ID, Repository: req.Tag, Run: &config})
	// if err != nil {
	// 	return nil, ErrBuildFailed
	// }

	// if h.debug {
	// 	log.Printf("Built image: %+v\n", builtImage)
	// }

	// temporary hack to work around bug in go-dockerclient
	err = h.commitContainerWithCli(container.ID, req.Tag, cmdEnv)
	if err != nil {
		return nil, err
	}

	return &BuildResult{true, nil}, nil
}

func (h requestHandler) commitContainerWithCli(id, tag string, env []string) error {
	c := exec.Command("/usr/bin/docker", "commit", `-run={"Cmd": ["/usr/bin/run"]}`, id, tag)
	var out, stdErr bytes.Buffer
	c.Stdout = &out
	c.Stderr = &stdErr

	err := c.Run()
	if h.debug {
		log.Printf("Commit output: %s\n", out.String())
		log.Printf("Commit stderr: %s\n", stdErr.String())
	}
	if err != nil {
		return err
	}

	return nil
}
