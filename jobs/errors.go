package jobs

var (
	ErrRanToCompletion = SimpleError{ResponseError, "This job has run to completion."}
)

const (
	ResponseOk ResponseSuccess = iota
	ResponseAccepted
)

const (
	ResponseError ResponseFailure = iota
	ResponseAlreadyExists
	ResponseNotFound
	ResponseInvalidRequest
	ResponseRateLimit
	ResponseNotAcceptable
)

// An error with a code and message to user
type SimpleError struct {
	Failure ResponseFailure
	Reason  string
}

func (j SimpleError) Error() string {
	return j.Reason
}

func (j SimpleError) ResponseFailure() ResponseFailure {
	return j.Failure
}

func (j SimpleError) ResponseData() interface{} {
	return nil
}

// An error that has associated response data to communicate
// to a client.
type StructuredJobError struct {
	SimpleError
	Data interface{}
}

func (j StructuredJobError) ResponseData() interface{} {
	return j.Data
}

// Cast error to UnknownJobError for default behavior
type UnknownJobError struct {
	error
}

func (s UnknownJobError) ResponseFailure() ResponseFailure {
	return ResponseError
}

func (s UnknownJobError) ResponseData() interface{} {
	return nil
}
