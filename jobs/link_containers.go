package jobs

import (
	"errors"
	"github.com/smarterclayton/geard/gears"
)

type GearLink struct {
	Gear         gears.Identifier
	NetworkLinks gears.NetworkLinks `json:"network_links"`
}

func (g *GearLink) Check() error {
	if g.Gear == "" {
		return errors.New("Gear identifier may not be empty")
	}
	if _, err := gears.NewIdentifier(string(g.Gear)); err != nil {
		return err
	}
	for i := range g.NetworkLinks {
		if err := g.NetworkLinks[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

type ExtendedLinkContainersData struct {
	Links []GearLink
}

func (g *ExtendedLinkContainersData) Check() error {
	if len(g.Links) == 0 {
		return errors.New("One or more gear links must be specified.")
	}
	for i := range g.Links {
		if err := g.Links[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

type LinkContainersRequest struct {
	JobResponse
	JobRequest
	Data *ExtendedLinkContainersData
}

func (j *LinkContainersRequest) Execute() {
	data := j.Data

	for i := range data.Links {
		if errw := data.Links[i].NetworkLinks.Write(data.Links[i].Gear.NetworkLinksPathFor(), false); errw != nil {
			j.Failure(ErrLinkContainersFailed)
			return
		}
	}

	j.Success(JobResponseOk)
}
