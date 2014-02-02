package geard

import (
	"errors"
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
			id := tracker.job.Id()
			log.Printf("job START %v", id)
			tracker.job.Execute()
			log.Printf("job END   %v", id)
			close(tracker.complete)
			d.recentJobs.Put(id, nil)
		}
	}()
}

type jobTracker struct {
	job      Job
	complete chan bool
}

func (d *Dispatcher) Dispatch(j Job) (done <-chan bool, err error) {
	complete := make(chan bool)
	tracker := jobTracker{j, complete}

	if existing, found := d.recentJobs.Put(j.Id(), tracker); found {
		var job Job
		if existing != nil {
			other, _ := existing.(jobTracker)
			job = other.job
			complete = other.complete
		}

		joined, complete, errj := j.Join(job, complete)
		if errj != nil {
			log.Println("Attempt to join job rejected ", j)
			err = errj
			return
		} else if joined {
			log.Println("Joined already running job ", job)
			done = complete
			return
		}
		log.Println("Queueing an already existing job ", j)
	}

	queue := d.slowJobs
	if j.Fast() {
		queue = d.fastJobs
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