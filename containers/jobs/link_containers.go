package jobs

import (
	"errors"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
)

type ContainerLink struct {
	Id           containers.Identifier
	NetworkLinks containers.NetworkLinks
}

func (link *ContainerLink) Check() error {
	if link.Id == "" {
		return errors.New("Container identifier may not be empty")
	}
	if _, err := containers.NewIdentifier(string(link.Id)); err != nil {
		return err
	}
	for i := range link.NetworkLinks {
		if err := link.NetworkLinks[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

type ContainerLinks struct {
	Links []ContainerLink
}

func (link *ContainerLinks) Check() error {
	if len(link.Links) == 0 {
		return errors.New("One or more links must be specified.")
	}
	for i := range link.Links {
		if err := link.Links[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

type LinkContainersRequest struct {
	*ContainerLinks
	Label string
}

func (j *LinkContainersRequest) JobLabel() string {
	return j.Label
}

func (j *LinkContainersRequest) Execute(resp jobs.JobResponse) {
	for i := range j.Links {
		if errw := j.Links[i].NetworkLinks.Write(j.Links[i].Id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrLinkContainersFailed)
			return
		}
	}

	resp.Success(jobs.JobResponseOk)
}
