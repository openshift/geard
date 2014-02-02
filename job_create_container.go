package geard

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"
)

type createContainerJobRequest struct {
	Request     jobRequest
	ContainerId string
	Image       string
	Output      io.Writer
}

func NewCreateContainerJob(reqid RequestIdentifier, id string, image string, input io.Reader, output io.Writer) (Job, error) {
	if reqid == nil {
		return nil, errors.New("All jobs must define a request id")
	}
	if id == "" {
		return nil, errors.New("A container must have an identifier")
	}
	if image == "" {
		return nil, errors.New("A container must have an image locator")
	}
	if input == nil {
		input = emptyReader
	}
	if output == nil {
		output = ioutil.Discard
	}
	return &createContainerJobRequest{jobRequest{reqid}, id, image, output}, nil
}

func (j *createContainerJobRequest) Id() RequestIdentifier {
	return j.Request.RequestId
}
func (j *createContainerJobRequest) Fast() bool {
	return false
}

func (j *createContainerJobRequest) Execute() {
	time.Sleep(8 * time.Second)
	fmt.Fprintf(j.Output, "Yo, I did your container create job %+v\n", j)
}

func (j *createContainerJobRequest) Join(job Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*createContainerJobRequest); !ok {
		if old == nil {
			err = ErrRanToCompletion
		} else {
			err = errors.New("Cannot join two jobs of different types.")
		}
		return
	}

	c := make(chan bool)
	done = c
	go func() {
		fmt.Fprintf(j.Output, "Joining an already running container create\n")
		time.Sleep(3 * time.Second)
		fmt.Fprintf(j.Output, "Other job must be done by now\n")
		close(c)
	}()
	joined = true
	return
}
