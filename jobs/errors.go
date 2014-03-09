package jobs

var (
	ErrRanToCompletion         = SimpleJobError{JobResponseError, "This job has run to completion."}
	ErrContainerNotFound       = SimpleJobError{JobResponseNotFound, "The specified container does not exist."}
	ErrContainerAlreadyExists  = SimpleJobError{JobResponseAlreadyExists, "A container with this identifier already exists."}
	ErrContainerCreateFailed   = SimpleJobError{JobResponseError, "Unable to create container."}
	ErrContainerStartFailed    = SimpleJobError{JobResponseError, "Unable to start this container."}
	ErrContainerStopFailed     = SimpleJobError{JobResponseError, "Unable to stop this container."}
	ErrContainerRestartFailed  = SimpleJobError{JobResponseError, "Unable to restart this container."}
	ErrEnvironmentNotFound     = SimpleJobError{JobResponseNotFound, "Unable to find the requested environment."}
	ErrEnvironmentUpdateFailed = SimpleJobError{JobResponseError, "Unable to update the specified environment."}
	ErrListImagesFailed        = SimpleJobError{JobResponseError, "Unable to list docker images."}
	ErrListContainersFailed    = SimpleJobError{JobResponseError, "Unable to list the installed containers."}
	ErrStartRequestThrottled   = SimpleJobError{JobResponseRateLimit, "It has been too soon since the last request to start."}
	ErrStopRequestThrottled    = SimpleJobError{JobResponseRateLimit, "It has been too soon since the last request to stop."}
	ErrRestartRequestThrottled = SimpleJobError{JobResponseRateLimit, "It has been too soon since the last request to restart or the state is currently changing."}
	ErrLinkContainersFailed    = SimpleJobError{JobResponseError, "Not all links could be set."}
	ErrDeleteContainerFailed   = SimpleJobError{JobResponseError, "Unable to delete the container."}
	ErrContentTypeDoesNotMatch = SimpleJobError{JobResponseNotAcceptable, "The content type you requested is not available for this action."}
)

const (
	JobResponseOk JobResponseSuccess = iota
	JobResponseAccepted
)

const (
	JobResponseError JobResponseFailure = iota
	JobResponseAlreadyExists
	JobResponseNotFound
	JobResponseInvalidRequest
	JobResponseRateLimit
	JobResponseNotAcceptable
)

// An error with a code and message to user
type SimpleJobError struct {
	Failure JobResponseFailure
	Reason  string
}

func (j SimpleJobError) Error() string {
	return j.Reason
}

func (j SimpleJobError) ResponseFailure() JobResponseFailure {
	return j.Failure
}

func (j SimpleJobError) ResponseData() interface{} {
	return nil
}

// An error that has associated response data to communicate
// to a client.
type StructuredJobError struct {
	SimpleJobError
	Data interface{}
}

func (j StructuredJobError) ResponseData() interface{} {
	return j.Data
}

// Cast error to UnknownJobError for default behavior
type UnknownJobError struct {
	error
}

func (s UnknownJobError) ResponseFailure() JobResponseFailure {
	return JobResponseError
}

func (s UnknownJobError) ResponseData() interface{} {
	return nil
}
