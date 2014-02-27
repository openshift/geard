package jobs

var (
	ErrRanToCompletion         = SimpleJobError{JobResponseError, "This job has run to completion."}
	ErrGearNotFound            = SimpleJobError{JobResponseNotFound, "The specified gear does not exist."}
	ErrGearAlreadyExists       = SimpleJobError{JobResponseAlreadyExists, "A gear with this identifier already exists."}
	ErrGearCreateFailed        = SimpleJobError{JobResponseError, "Unable to create gear."}
	ErrRepositoryAlreadyExists = SimpleJobError{JobResponseAlreadyExists, "A repository with this identifier already exists."}
	ErrSubscribeToUnit         = SimpleJobError{JobResponseError, "Unable to watch for the completion of this action."}
	ErrRepositoryCreateFailed  = SimpleJobError{JobResponseError, "Unable to create the repository."}
	ErrGearStartFailed         = SimpleJobError{JobResponseError, "Unable to start this gear."}
	ErrGearStopFailed          = SimpleJobError{JobResponseError, "Unable to stop this gear."}
	ErrEnvironmentUpdateFailed = SimpleJobError{JobResponseError, "Unable to update the specified environment."}
	ErrListImagesFailed        = SimpleJobError{JobResponseError, "Unable to list docker images."}
	ErrListContainersFailed    = SimpleJobError{JobResponseError, "Unable to list the installed containers."}
	ErrStartRequestThrottled   = SimpleJobError{JobResponseRateLimit, "It has been too soon since the last request to start."}
	ErrStopRequestThrottled    = SimpleJobError{JobResponseRateLimit, "It has been too soon since the last request to stop."}
	ErrLinkContainersFailed    = SimpleJobError{JobResponseError, "Not all links could be set."}
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
