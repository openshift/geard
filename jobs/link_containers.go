package jobs

import (
	"errors"
	"github.com/smarterclayton/geard/containers"
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

type ExtendedLinkContainersData struct {
	Links []ContainerLink
}

func (link *ExtendedLinkContainersData) Check() error {
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
	Data *ExtendedLinkContainersData
}

func (j *LinkContainersRequest) Execute(resp JobResponse) {
	data := j.Data

	for i := range data.Links {
		if errw := data.Links[i].NetworkLinks.Write(data.Links[i].Id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrLinkContainersFailed)
			return
		}
	}

	resp.Success(JobResponseOk)
}
