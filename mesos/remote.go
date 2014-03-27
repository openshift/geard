// +build mesos

package mesos

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/kraman/mesos-go/src/mesos.apache.org/mesos"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/jobs"

	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
)

type Locator interface {
	IsRemote() bool
	Identity() string
}

type RemoteLocator interface {
	BaseURL() *url.URL
}

type RemoteJob interface {
	jobs.Job
	RequestName() string
	Method() string
	MarshalRequestBody() ([]byte, error)
	MarshalRequestIdentifier() jobs.RequestIdentifier
	UnmarshalRequestBody(data []byte, taskId jobs.RequestIdentifier) error
}

type RemoteExecutable interface {
	RemoteJob
}

type MesosDispatcher struct {
	locator      RemoteLocator
	log          *log.Logger
	jobQueue     chan RemoteExecutable
	responseChan chan MesosJobResponse
	errorChan    chan error
}

func NewMesosDispatcher(l RemoteLocator, logger *log.Logger) *MesosDispatcher {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	return &MesosDispatcher{
		locator:      l,
		log:          logger,
		jobQueue:     make(chan RemoteExecutable, 1),
		responseChan: make(chan MesosJobResponse),
		errorChan:    make(chan error),
	}
}

func (m *MesosDispatcher) Dispatch(job RemoteExecutable, res jobs.JobResponse) (err error) {
	m.jobQueue <- job

	masterHost := m.locator.BaseURL().Host
	driver := mesos.SchedulerDriver{
		Master: masterHost,
		Framework: mesos.FrameworkInfo{
			Name: proto.String("Gear CLI"),
			User: proto.String(""),
		},

		Scheduler: &mesos.Scheduler{
			ResourceOffers: m.processResourceOffer,
			StatusUpdate:   m.processJobStatusUpdate,
		},
	}

	driver.Init()
	defer driver.Destroy()

	driver.Start()
	select {
	case err = <-m.errorChan:
		log.Println("Job error", err)
	case jobResponse := <-m.responseChan:
		if jobResponse.Succeeded {
			res.SuccessWithData(jobResponse.JobResponse, jobResponse.Data)
		} else {
			res.Failure(jobResponse.FailureReason)
		}
	}

	driver.Stop(false)
	return
}

// Invoked when the status of a task has changed (e.g., a slave is lost and so the task is lost, a task finishes and an executor
// sends a status update saying so, etc). Note that returning from this callback _acknowledges_ receipt of this status update!
// If for whatever reason the scheduler aborts during this callback (or the process exits) another status update will be delivered.
func (m *MesosDispatcher) processJobStatusUpdate(driver *mesos.SchedulerDriver, status mesos.TaskStatus) {
	//log.Printf("Received task status: %v\n", status)
	state := *status.State
	if state == mesos.TaskState_TASK_FINISHED || state == mesos.TaskState_TASK_FAILED {
		if status.GetData() != nil {
			data := status.GetData()
			jobResponse := MesosJobResponse{}
			err := json.Unmarshal(data, &jobResponse)
			if err != nil {
				m.errorChan <- err
			}
			m.responseChan <- jobResponse
		} else {
			m.errorChan <- fmt.Errorf(*status.Message)
		}
	}
}

// Invoked when resources have been offered to this framework. A single offer will only contain resources from a single slave.
// Resources associated with an offer will not be re-offered to _this_ framework until either (a) this framework has rejected
// those resources (see SchedulerDriver::launchTasks) or (b) those resources have been rescinded (see Scheduler::offerRescinded).
// Note: Resources may be concurrently offered to more than one framework at a time (depending on the allocator being used). In
// that case, the first framework to launch tasks using those resources will be able to use them. The other frameworks will have
// those resources rescinded (or if a framework has already launched tasks with those resources then those tasks will fail with a
// TASK_LOST status and a message saying as much).
func (m *MesosDispatcher) processResourceOffer(driver *mesos.SchedulerDriver, offers []mesos.Offer) {
	job := <-m.jobQueue

	executor, err := m.getExecutor(job)
	if err != nil {
		m.errorChan <- err
		return
	}

	id := job.MarshalRequestIdentifier()
	if len(id) == 0 {
		id = jobs.NewRequestIdentifier()
	}

	for _, offer := range offers {
		//log.Printf("Launching task: %v\n", id.Exact())

		tasks := []mesos.TaskInfo{
			{
				Name: proto.String(job.RequestName()),
				TaskId: &mesos.TaskID{
					Value: proto.String(id.Exact()),
				},
				SlaveId:  offer.SlaveId,
				Executor: executor,
				Resources: []*mesos.Resource{
					mesos.ScalarResource("cpus", 1),
					mesos.ScalarResource("mem", 100),
				},
			},
		}

		driver.LaunchTasks(offer.Id, tasks)
	}
}

func (m *MesosDispatcher) getExecutor(job RemoteExecutable) (executor *mesos.ExecutorInfo, err error) {
	data, err := job.MarshalRequestBody()
	if err != nil {
		return
	}

	exec_path := path.Join(config.ContainerBasePath(), "bin", "gear-mesos-executor")
	executor = &mesos.ExecutorInfo{
		ExecutorId: &mesos.ExecutorID{Value: proto.String("default")},
		Command: &mesos.CommandInfo{
			Value: proto.String(exec_path),
			Uris: []*mesos.CommandInfo_URI{
				{Value: &exec_path},
			},
		},
		Name:   proto.String(job.Method()),
		Source: proto.String("Gear Mesos Executor"),
		Data:   data,
	}
	return
}
