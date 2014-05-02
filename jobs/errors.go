package jobs

var (
	ErrRanToCompletion = SimpleJobError{JobResponseError, "This job has run to completion."}
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
