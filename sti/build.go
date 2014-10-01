package sti

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/fsouza/go-dockerclient"
)

type buildRequestHandler struct {
	*requestHandler
}

// Build processes a Request and returns a *Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
//
func Build(req *STIRequest) (*STIResult, error) {
	rh, err := newHandler(req)
	if err != nil {
		return nil, err
	}
	defer rh.cleanup()
	h := &buildRequestHandler{requestHandler: rh}
	h.postExecutor = h

	err = h.setup([]string{"assemble", "run"}, []string{"save-artifacts"}, "assemble")
	if err != nil {
		return nil, err
	}

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

	err = h.getSource()
	if err != nil {
		return nil, err
	}

	err = h.execute("assemble")
	if err != nil {
		return nil, err
	}

	return h.result, nil
}

func (h *buildRequestHandler) postExecute(container *docker.Container) error {
	config := docker.Config{
		Cmd: []string{"/tmp/scripts/run"},
		Env: h.generateConfigEnv(),
	}

	previousImageId := ""
	var err error
	if h.request.incremental && h.request.RemovePreviousImage {
		previousImageId, err = h.getPreviousImageId()
		if err != nil {
			log.Printf("Error retrieving previous image's metadata: %s", err.Error())
		}
	}

	log.Println("Committing container")
	if h.request.Verbose {
		log.Printf("Commiting container with config: %+v\n", config)
	}

	builtImage, err := h.dockerClient.CommitContainer(docker.CommitContainerOptions{Container: container.ID, Repository: h.request.Tag, Run: &config})
	if err != nil {
		return ErrBuildFailed
	}

	h.result.ImageID = builtImage.ID
	log.Printf("Tagged %s as %s\n", builtImage.ID, h.request.Tag)

	if h.request.incremental && h.request.RemovePreviousImage && previousImageId != "" {
		log.Printf("Removing previously-tagged image %s\n", previousImageId)
		err = h.dockerClient.RemoveImage(previousImageId)
		if err != nil {
			log.Printf("Unable to remove previous image: %s\n", err.Error())
		}
	}

	if h.request.CallbackUrl != "" {
		executeCallback(h.request.CallbackUrl, h.result)
	}

	return nil
}

func (h *buildRequestHandler) getPreviousImageId() (string, error) {
	imageMetadata, err := h.dockerClient.InspectImage(h.request.Tag)
	if err == nil {
		return imageMetadata.ID, nil
	}
	return "", err
}

func (h *buildRequestHandler) determineIncremental() error {
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
		_, err := os.Stat(filepath.Join(h.request.workingDir, "upload/scripts/save-artifacts"))
		incremental = (err == nil)
	}

	h.request.incremental = incremental

	return nil
}

func (h *buildRequestHandler) saveArtifacts() error {
	artifactTmpDir := filepath.Join(h.request.workingDir, "upload/artifacts")
	err := os.Mkdir(artifactTmpDir, 0700)
	if err != nil {
		return err
	}

	image := h.request.Tag

	log.Printf("Saving build artifacts from image %s to path %s\n", image, artifactTmpDir)

	config := docker.Config{
		Image:        image,
		Cmd:          []string{"/tmp/scripts/save-artifacts"},
		AttachStdout: true,
	}
	if h.request.Verbose {
		log.Printf("Creating container using config: %+v\n", config)
	}
	container, err := h.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return err
	}
	defer h.removeContainer(container.ID)

	reader, writer := io.Pipe()

	if h.request.Verbose {
		log.Printf("Attaching to container")
	}
	attached := make(chan struct{})
	attachOpts := docker.AttachToContainerOptions{
		Container:    container.ID,
		Stdout:       true,
		OutputStream: writer,
		Stream:       true,
		Success:      attached,
	}
	go h.dockerClient.AttachToContainer(attachOpts)
	// this lets dockerClient know that we're ready to receive data
	attached <- <-attached

	if h.request.Verbose {
		log.Printf("Starting container")
	}
	err = h.dockerClient.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}

	if h.request.Verbose {
		log.Printf("Reading artifacts tar stream")
	}

	err = h.extractTarStream(artifactTmpDir, reader)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	err = reader.Close()
	if err != nil {
		return err
	}

	if h.request.Verbose {
		log.Printf("Waiting for container to exit")
	}
	exitCode, err := h.dockerClient.WaitContainer(container.ID)
	if err != nil {
		return err
	}
	if h.request.Verbose {
		log.Printf("Container exited")
	}

	if exitCode != 0 {
		if h.request.Verbose {
			log.Printf("Exit code: %d", exitCode)
		}
		return ErrSaveArtifactsFailed
	}

	return nil
}

func (h *buildRequestHandler) getSource() error {
	targetSourceDir := filepath.Join(h.request.workingDir, "upload", "src")

	log.Printf("Downloading %s to directory %s\n", h.request.Source, targetSourceDir)

	if validCloneSpec(h.request.Source) {
		err := gitClone(h.request.Source, targetSourceDir)
		if err != nil {
			log.Printf("Git clone failed: %+v", err)
			return err
		}

		if h.request.Ref != "" {
			log.Printf("Checking out ref %s", h.request.Ref)

			err := gitCheckout(targetSourceDir, h.request.Ref, h.request.Verbose)
			if err != nil {
				return err
			}
		}
	} else {
		copy(h.request.Source, targetSourceDir)
	}

	return nil
}
