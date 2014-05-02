package jobs

import (
	"github.com/openshift/geard/jobs"
)

var (
	ErrContainerNotFound       = jobs.SimpleJobError{jobs.JobResponseNotFound, "The specified container does not exist."}
	ErrContainerAlreadyExists  = jobs.SimpleJobError{jobs.JobResponseAlreadyExists, "A container with this identifier already exists."}
	ErrContainerCreateFailed   = jobs.SimpleJobError{jobs.JobResponseError, "Unable to create container."}
	ErrContainerStartFailed    = jobs.SimpleJobError{jobs.JobResponseError, "Unable to start this container."}
	ErrContainerStopFailed     = jobs.SimpleJobError{jobs.JobResponseError, "Unable to stop this container."}
	ErrContainerRestartFailed  = jobs.SimpleJobError{jobs.JobResponseError, "Unable to restart this container."}
	ErrEnvironmentNotFound     = jobs.SimpleJobError{jobs.JobResponseNotFound, "Unable to find the requested environment."}
	ErrEnvironmentUpdateFailed = jobs.SimpleJobError{jobs.JobResponseError, "Unable to update the specified environment."}
	ErrListImagesFailed        = jobs.SimpleJobError{jobs.JobResponseError, "Unable to list docker images."}
	ErrListContainersFailed    = jobs.SimpleJobError{jobs.JobResponseError, "Unable to list the installed containers."}
	ErrStartRequestThrottled   = jobs.SimpleJobError{jobs.JobResponseRateLimit, "It has been too soon since the last request to start."}
	ErrStopRequestThrottled    = jobs.SimpleJobError{jobs.JobResponseRateLimit, "It has been too soon since the last request to stop."}
	ErrRestartRequestThrottled = jobs.SimpleJobError{jobs.JobResponseRateLimit, "It has been too soon since the last request to restart or the state is currently changing."}
	ErrLinkContainersFailed    = jobs.SimpleJobError{jobs.JobResponseError, "Not all links could be set."}
	ErrDeleteContainerFailed   = jobs.SimpleJobError{jobs.JobResponseError, "Unable to delete the container."}
)
