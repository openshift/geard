package init

import (
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"os"
)

type DataContainerPattern struct {
	*docker.Client
	Hooks []ContainerHook
}

type ContainerHook interface {
	Update(image docker.Image, opt docker.CreateContainerOptions) (docker.CreateContainerOptions, error)
	Alter(container docker.Container, client *docker.Client) error
}

var ErrImageRemoved = errors.New("the requested image was removed from the system")

func isNoSuchContainer(err error) bool {
	switch err.(type) {
	case *docker.NoSuchContainer:
		return true
	}
	return false
}

func (c *DataContainerPattern) Create(opt docker.CreateContainerOptions) error {
	pull := false

	existing, err := c.InspectContainer(opt.Name)
	if err != nil && !isNoSuchContainer(err) {
		return err
	}
	if existing != nil {
		if err := c.KillContainer(docker.KillContainerOptions{ID: existing.ID}); err != nil && !isNoSuchContainer(err) {
			return err
		}
		if err := c.RemoveContainer(docker.RemoveContainerOptions{ID: opt.Name}); err != nil && !isNoSuchContainer(err) {
			return err
		}
		existing = nil
	}

	// pull the image if necessary
	image, err := c.InspectImage(opt.Config.Image)
	if err == docker.ErrNoSuchImage {
		pull = true
	} else if err != nil {
		return err
	}
	if pull {
		fmt.Fprintf(os.Stderr, "Container image needs to be downloaded '%s' ... ", opt.Config.Image)
		if err := c.PullImage(docker.PullImageOptions{opt.Config.Image, "", "", os.Stdout}, docker.AuthConfiguration{}); err != nil {
			return err
		}

		image, err = c.InspectImage(opt.Config.Image)
		if err == docker.ErrNoSuchImage {
			return ErrImageRemoved
		} else if err != nil {
			return err
		}
	}

	// create a data volume if the image exposes volumes
	if len(image.ContainerConfig.Volumes) != 0 {
		opt.Config.VolumesFrom = opt.Name + "-data"
		dataContainerOpts := docker.CreateContainerOptions{
			Name: opt.Config.VolumesFrom,
			Config: &docker.Config{
				Image: opt.Config.Image,
				Cmd:   []string{"true"},
			},
		}
		if _, err := c.CreateContainer(dataContainerOpts); err != nil {
			if err == docker.ErrNoSuchImage {
				return ErrImageRemoved
			}
			return fmt.Errorf("the data volumes for this container could not be created: %s", err.Error())
		}
	}

	for i := range c.Hooks {
		override, err := c.Hooks[i].Update(*image, opt)
		if err != nil {
			return err
		}
		opt = override
	}

	// create the active container
	if existing, err = c.CreateContainer(opt); err != nil {
		if err == docker.ErrNoSuchImage {
			return ErrImageRemoved
		}
		return err
	}

	for i := range c.Hooks {
		err := c.Hooks[i].Alter(*existing, c.Client)
		if err != nil {
			return err
		}
	}

	return nil
}
