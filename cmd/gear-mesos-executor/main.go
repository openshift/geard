// +build mesos

package main

import (
	"github.com/kraman/mesos-go/src/mesos.apache.org/mesos"

	"github.com/openshift/geard/containers"
	gearexec "github.com/openshift/geard/mesos/executor"
	"github.com/openshift/geard/systemd"

	"fmt"
	"os"
)

func main() {
	if os.Getenv("MESOS_SLAVE_PID") == "" {
		fmt.Printf("Gear Mesos executor must be run by a Mesos slave. It is not a standalone command.\n")
		os.Exit(1)
	}

	systemd.Require()
	err := containers.InitializeData()
	if err != nil {
		fmt.Printf("Unable to initialize container data dir.\n")
		os.Exit(2)
	}

	driver := mesos.ExecutorDriver{Executor: gearexec.NewGearExecutor()}

	driver.Init()
	defer driver.Destroy()
	driver.Run()
}
