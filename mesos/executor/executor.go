// +build mesos

package executor

import (
	"code.google.com/p/goprotobuf/proto"
	m "github.com/kraman/mesos-go/src/mesos.apache.org/mesos"
	j "github.com/openshift/geard/jobs"
	"github.com/openshift/geard/mesos"
	"github.com/openshift/geard/mesos/jobs"

	"encoding/json"
	"fmt"
	"log"
)

func NewGearExecutor() *m.Executor {
	executor := m.Executor{}
	executor.Registered = registered
	executor.Reregistered = reregistered
	executor.Disconnected = disconnected
	executor.LaunchTask = launchTask
	executor.KillTask = killTask
	executor.FrameworkMessage = frameworkMessage
	executor.Shutdown = shutdown
	executor.Error = fatalError
	return &executor
}

func sendStatusUpdate(driver *m.ExecutorDriver, taskId *m.TaskID, state m.TaskState, message string) {
	driver.SendStatusUpdate(&m.TaskStatus{
		TaskId:  taskId,
		State:   m.NewTaskState(state),
		Message: proto.String(message),
	})
}

func sendStatusUpdateWithData(driver *m.ExecutorDriver, taskId *m.TaskID, state m.TaskState, message string, data []byte) {
	driver.SendStatusUpdate(&m.TaskStatus{
		TaskId:  taskId,
		State:   m.NewTaskState(state),
		Message: proto.String(message),
		Data:    data,
	})
}

// Invoked once the executor driver has been able to successfully connect with Mesos.
// A scheduler can pass some data to its executors through the executor::GetData().
func registered(driver *m.ExecutorDriver, executor m.ExecutorInfo, framework m.FrameworkInfo, slave m.SlaveInfo) {
	log.Printf("Executor %v(%v) for framework %v(%v) registered on slave %v:%v (%v)",
		executor.GetName(), executor.GetExecutorId(),
		framework.GetName(), framework.GetId(),
		slave.GetHostname(), slave.GetPort(), slave.GetId())
}

// Invoked when the executor re-registers with a restarted slave
func reregistered(driver *m.ExecutorDriver, slace m.SlaveInfo) {
}

//Invoked when the executor becomes "disconnected" from the slave (e.g., the slave is being restarted due to an upgrade).
func disconnected(driver *m.ExecutorDriver) {
}

// Invoked when a task has been launched on this executor (initiated via Scheduler::LaunchTasks).
// Note: No other callbacks will be invoked on this executor until this function has returned.
func launchTask(driver *m.ExecutorDriver, taskInfo m.TaskInfo) {
	var job mesos.RemoteJob
	cmd := taskInfo.GetExecutor().GetName()

	sendStatusUpdate(driver, taskInfo.TaskId, m.TaskState_TASK_STARTING, "")
	log.Printf("Launch task { Name '%v', Id '%v', Slave ID '%v', Command '%v' }", taskInfo.GetName(), taskInfo.GetTaskId(), taskInfo.GetSlaveId(), cmd)
	data := taskInfo.GetExecutor().GetData()

	switch {
	case cmd == "MesosInstallContainerRequest":
		job = &jobs.MesosInstallContainerRequest{}
	case cmd == "MesosDeleteContainerRequest":
		job = &jobs.MesosDeleteContainerRequest{}
	default:
		sendStatusUpdate(driver, taskInfo.TaskId, m.TaskState_TASK_FAILED, "Unknown job command")
		return
	}

	id := taskInfo.GetTaskId().GetValue()
	reqId, err := j.NewRequestIdentifierFromString(id)
	if err != nil {
		str := fmt.Sprintf("Error while unmarshaling request id: %v", err)
		log.Println(str)
		sendStatusUpdate(driver, taskInfo.TaskId, m.TaskState_TASK_FAILED, str)
	}

	err = job.UnmarshalRequestBody(data, reqId)
	if err != nil {
		str := fmt.Sprintf("Error while unmarshaling request: %v", err)
		log.Println(str)
		sendStatusUpdate(driver, taskInfo.TaskId, m.TaskState_TASK_FAILED, str)
	}

	sendStatusUpdate(driver, taskInfo.TaskId, m.TaskState_TASK_RUNNING, "")
	jobResponse := mesos.MesosJobResponse{}
	job.Execute(&jobResponse)

	data, err = json.Marshal(jobResponse)
	if err != nil {
		log.Printf("Error marshalling jobResponse: %v", err)
	}

	if jobResponse.Succeeded {
		log.Printf("Launch task succesfully { Name '%v', Id '%v', Slave ID '%v' }", taskInfo.GetName(), taskInfo.GetTaskId(), taskInfo.GetSlaveId())
		sendStatusUpdateWithData(driver, taskInfo.TaskId, m.TaskState_TASK_FINISHED, "Completed task", data)
	}
	if jobResponse.Failed {
		log.Printf("Failed task { Name '%v', Id '%v', Slave ID '%v' }", taskInfo.GetName(), taskInfo.GetTaskId(), taskInfo.GetSlaveId())
		sendStatusUpdateWithData(driver, taskInfo.TaskId, m.TaskState_TASK_FAILED, "Completed task", data)
	}
}

// Invoked when a task running within this executor has been killed (via SchedulerDriver::KillTask).
// Note: No status update will be sent on behalf of the executor, the executor is responsible
// for creating a new TaskStatus (i.e., with TASK_KILLED) and invoking ExecutorDriver::sendStatusUpdate.
func killTask(driver *m.ExecutorDriver, taskId m.TaskID) {
}

// Invoked when a framework message has arrived for this executor. These messages are best effort
func frameworkMessage(driver *m.ExecutorDriver, message string) {
}

// Invoked when the executor should terminate all of its currently running tasks.
// Note: After Mesos has determined that an executor has terminated, any tasks that the executor did not send
// a terminal status updates for (e.g., TASK_KILLED, TASK_FINISHED, TASK_FAILED, etc) will be marked as TASK_LOST.
func shutdown(driver *m.ExecutorDriver) {
}

// Invoked when a fatal error has occured with the executor and/or executor driver. The driver will be aborted BEFORE invoking this callback.
func fatalError(driver *m.ExecutorDriver, errorMsg string) {
}
