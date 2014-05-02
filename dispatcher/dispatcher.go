// Queue manager for executing jobs, handling deduplication of requests, and
// limiting the consumption of resources by a server.  Allows callers to
// "join" an existing job and listen to the output provided by that job without
// reexecuting the actual task.
package dispatcher

import (
	"errors"
	"github.com/openshift/geard/jobs"
	"log"
)

type Dispatcher struct {
	QueueFast         int
	QueueSlow         int
	Concurrent        int
	TrackDuplicateIds int

	fastJobs   chan jobTracker
	slowJobs   chan jobTracker
	recentJobs *RequestIdentifierMap
}

type Fast interface {
	Fast() bool
}

func (d *Dispatcher) Start() {
	d.recentJobs = NewRequestIdentifierMap(d.TrackDuplicateIds)
	d.fastJobs = make(chan jobTracker, d.QueueFast)
	d.slowJobs = make(chan jobTracker, d.QueueSlow)
	for i := 0; i < d.Concurrent; i++ {
		d.work(d.fastJobs)
		d.work(d.slowJobs)
	}
}

func (d *Dispatcher) work(queue <-chan jobTracker) {
	go func() {
		for tracker := range queue {
			id := tracker.id
			log.Printf("job START %s: %+v", id.String(), tracker.job)
			tracker.job.Execute(tracker.response)
			log.Printf("job END   %s", id.String())
			close(tracker.complete)
			d.recentJobs.Put(id, nil)
		}
	}()
}

type jobTracker struct {
	id       jobs.RequestIdentifier
	job      jobs.Job
	response jobs.Response
	complete chan bool
}

func (d *Dispatcher) Dispatch(id jobs.RequestIdentifier, j jobs.Job, resp jobs.Response) (done <-chan bool, err error) {
	complete := make(chan bool)
	tracker := jobTracker{id, j, resp, complete}

	if existing, found := d.recentJobs.Put(id, tracker); found {
		var join jobs.Join
		if existing != nil {
			other, _ := existing.(jobTracker)
			j, ok := other.job.(jobs.Join)
			if !ok {
				err = jobs.ErrRanToCompletion
				return
			}
			join = j
			complete = other.complete
		} else {
			self, ok := j.(jobs.Join)
			if !ok {
				err = jobs.ErrRanToCompletion
				return
			}
			join = self
		}

		joined, complete, errj := join.Join(j, complete)
		if errj != nil {
			log.Println("Attempt to join job rejected ", j)
			err = errj
			return
		} else if joined {
			log.Println("Joined already running job ", j)
			done = complete
			return
		}
		log.Println("Queueing an already existing job ", j)
	}

	var queue chan jobTracker
	fast := false
	if f, ok := j.(Fast); ok {
		fast = f.Fast()
	}
	if fast {
		queue = d.fastJobs
	} else {
		queue = d.slowJobs
	}

	select {
	case queue <- tracker:
	default:
		err = errors.New("The server is at maximum capacity - please try again shortly")
		return
	}

	done = complete
	return
}

func closedChannel() <-chan bool {
	c := make(chan bool)
	close(c)
	return c
}
