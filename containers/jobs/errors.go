package jobs

import (
	"github.com/openshift/geard/jobs"
)

var (
	ErrContainerNotFound       = jobs.SimpleError{jobs.ResponseNotFound, "The specified container does not exist."}
	ErrContainerAlreadyExists  = jobs.SimpleError{jobs.ResponseAlreadyExists, "A container with this identifier already exists."}
	ErrContainerStartFailed    = jobs.SimpleError{jobs.ResponseError, "Unable to start this container."}
	ErrContainerStopFailed     = jobs.SimpleError{jobs.ResponseError, "Unable to stop this container."}
	ErrContainerRestartFailed  = jobs.SimpleError{jobs.ResponseError, "Unable to restart this container."}
	ErrEnvironmentNotFound     = jobs.SimpleError{jobs.ResponseNotFound, "Unable to find the requested environment."}
	ErrEnvironmentUpdateFailed = jobs.SimpleError{jobs.ResponseError, "Unable to update the specified environment."}
	ErrListImagesFailed        = jobs.SimpleError{jobs.ResponseError, "Unable to list docker images."}
	ErrListContainersFailed    = jobs.SimpleError{jobs.ResponseError, "Unable to list the installed containers."}
	ErrStartRequestThrottled   = jobs.SimpleError{jobs.ResponseRateLimit, "It has been too soon since the last request to start."}
	ErrStopRequestThrottled    = jobs.SimpleError{jobs.ResponseRateLimit, "It has been too soon since the last request to stop."}
	ErrRestartRequestThrottled = jobs.SimpleError{jobs.ResponseRateLimit, "It has been too soon since the last request to restart or the state is currently changing."}
	ErrLinkContainersFailed    = jobs.SimpleError{jobs.ResponseError, "Not all links could be set."}
	ErrDeleteContainerFailed   = jobs.SimpleError{jobs.ResponseError, "Unable to delete the container."}

	ErrContainerCreateFailed              = jobs.SimpleError{jobs.ResponseError, "Unable to create container."}
	ErrContainerCreateFailedPortsReserved = jobs.SimpleError{jobs.ResponseError, "Unable to create container: some ports could not be reserved."}
)
