package jobs

import (
	"errors"
)

// No job exists that matches the request.
var ErrNoJobForRequest = errors.New("no job matches that request")

// Interface for mapping between a request and a Job
type JobExtension interface {
	// Return an error if the requested job is invalid, ErrNoJobForRequest
	// if this interface does not recognize the Job, or a valid Job.
	JobFor(request interface{}) (Job, error)
}

// Convenience wrapper for execution a JobExtension handler
type JobExtensionFunc func(interface{}) (Job, error)

func (f JobExtensionFunc) JobFor(request interface{}) (Job, error) {
	return f(request)
}

// All local execution extensions
var extensions []JobExtension

// Register a job extension to this executable during init() or startup
func AddJobExtension(extension JobExtension) {
	extensions = append(extensions, extension)
}

// Return a registered Job implementation that satisfies the
// requested job. If no such implementation exists, returns
// ErrNoJobForRequest.
func JobFor(request interface{}) (Job, error) {
	for i := range extensions {
		job, err := extensions[i].JobFor(request)
		if err == ErrNoJobForRequest {
			continue
		}
		if err != nil {
			return nil, err
		}
		return job, err
	}
	return nil, ErrNoJobForRequest
}
