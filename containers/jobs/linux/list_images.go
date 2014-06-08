package linux

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"

	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
)

type listImages struct {
	*ListImagesRequest
}

func (j *listImages) Execute(resp jobs.Response) {
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
