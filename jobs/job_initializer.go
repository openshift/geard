package jobs

import (
	"github.com/openshift/geard/utils"
)

type JobInitializer struct {
	Extension JobExtension
	Func      func() error

	once utils.ErrorOnce
}

func (c *JobInitializer) JobFor(request interface{}) (Job, error) {
	if err := c.once.Error(c.Func); err != nil {
		return nil, err
	}
	return c.Extension.JobFor(request)
}
