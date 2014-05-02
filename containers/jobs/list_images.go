package jobs

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/geard/jobs"
	"log"
)

type ListImagesRequest struct {
	DockerSocket string
}

func (j *ListImagesRequest) Execute(resp jobs.Response) {
	// TODO: config item for docker port
	dockerClient, err := docker.NewClient(j.DockerSocket)

	if err != nil {
		log.Printf("job_list_images: Couldn't connect to docker: %+v", err)
		resp.Failure(ErrListImagesFailed)
		return
	}

	imgs, err := dockerClient.ListImages(false)

	if err != nil {
		log.Printf("job_list_images: Couldn't connect to docker: %+v", err)
		resp.Failure(ErrListImagesFailed)
		return
	}

	w := resp.SuccessWithWrite(jobs.ResponseAccepted, true, false)
	for _, img := range imgs {
		fmt.Fprintf(w, "%+v\n", img.RepoTags[0])
	}
}
