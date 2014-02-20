package jobs

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
)

type ListImagesRequest struct {
	JobResponse
	JobRequest
}

func (j *ListImagesRequest) Execute() {
	w := j.SuccessWithWrite(JobResponseAccepted, true)

	// TODO: config item for docker port
	dockerClient, err := docker.NewClient("unix:///var/run/docker.sock")

	if err != nil {
		fmt.Fprintf(w, "job_list_images: Couldn't connect to docker: %+v", err)
		j.Failure(ErrListImagesFailed)
		return
	}

	imgs, err := dockerClient.ListImages(false)

	if err != nil {
		fmt.Fprintf(w, "job_list_images: Couldn't connect to docker: %+v", err)
		j.Failure(ErrListImagesFailed)
		return
	}

	for _, img := range imgs {
		fmt.Fprintf(w, "%+v\n", img.RepoTags[0])
	}
}
