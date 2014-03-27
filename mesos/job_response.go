// +build mesos

package mesos

import (
	"bytes"
	"io"

	"github.com/openshift/geard/jobs"
)

type MesosJobResponse struct {
	pending map[string]interface{}

	Succeeded bool
	Failed    bool

	Data          interface{}
	FailureReason jobs.JobError
	JobResponse   jobs.JobResponseSuccess
}

func (r *MesosJobResponse) StreamResult() bool {
	return false
}

func (r *MesosJobResponse) Success(t jobs.JobResponseSuccess) {
	if r.Failed {
		panic("Cannot call Success() after failure")
	}
	if r.Succeeded {
		panic("Cannot call Success() twice")
	}

	if r.pending != nil {
		r.Data = r.pending
	}
	r.Succeeded = true
	r.JobResponse = t
}

func (r *MesosJobResponse) SuccessWithData(t jobs.JobResponseSuccess, data interface{}) {
	r.Success(t)
	r.Data = data
}

func (r *MesosJobResponse) SuccessWithWrite(t jobs.JobResponseSuccess, flush, structured bool) io.Writer {
	r.Success(t)

	if structured {
		panic("Client does not support receiving streaming structured data.")
	}

	buf := bytes.Buffer{}
	r.Data = &buf
	return &buf
}

func (r *MesosJobResponse) Failure(reason jobs.JobError) {
	if r.Failed {
		panic("Cannot call Failure() twice")
	}
	if r.Succeeded {
		panic("Cannot call Failure() after Success()")
	}

	r.Failed = true
	r.FailureReason = reason
}

func (r *MesosJobResponse) WriteClosed() <-chan bool {
	ch := make(chan bool)
	return ch
}

func (r *MesosJobResponse) WritePendingSuccess(name string, value interface{}) {
	if r.pending == nil {
		r.pending = make(map[string]interface{})
	}

	r.pending[name] = value
}
