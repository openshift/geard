package cmd

import (
	"fmt"
	"github.com/smarterclayton/cobra"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/systemd"
	"log"
	"os"
	"strings"
	"sync"
)

func run(cmd *cobra.Command, init func(jobs.JobResponse) jobs.Job) {
	r := &CliJobResponse{cmd.Out(), cmd.Out(), false, false, 0, ""}
	j := init(r)
	j.Execute()
	if r.exitCode != 0 {
		if r.message == "" {
			r.message = "Command failed"
		}
		fail(r.exitCode, r.message)
	}
	os.Exit(r.exitCode)
}

func fail(code int, format string, other ...interface{}) {
	fmt.Fprintf(os.Stderr, format, other)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(code)
}

func Execute() {
	var (
		daemon bool
		conf   http.HttpConfiguration
	)

	gearCmd := &cobra.Command{
		Use:   "gear",
		Short: "Gear(d) is a tool for installing Docker containers to systemd",
		Long: `A commandline client and server that allows Docker containers to
              be installed to Systemd in an opinionated and distributed
              fashion.
              Complete documentation is available at http://github.com/smarterclayton/geard`,
		Run: func(cmd *cobra.Command, args []string) {
			if !daemon {
				cmd.Usage()
				return
			}

			systemd.Start()
			gears.InitializeData()
			var dispatch = dispatcher.Dispatcher{
				QueueFast:         10,
				QueueSlow:         1,
				Concurrent:        2,
				TrackDuplicateIds: 1000,
			}
			dispatch.Start()
			gears.StartPortAllocator(4000, 60000)
			wg := &sync.WaitGroup{}

			http.StartAPI(wg, conf, &dispatch)
			wg.Wait()
			log.Print("Exiting ...")
		},
	}
	gearCmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run as a server process")
	gearCmd.Flags().StringVarP(&(conf.DockerSocket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")
	gearCmd.Flags().StringVarP(&(conf.ListenAddr), "listen-address", "A", ":8080", "Set the address for the http endpoint to listen on")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Disable all gears, slices, and targets in systemd",
		Long: `Disable all registered resources from systemd to allow them to be
              removed from the system.  Will reload the systemd daemon config.`,
		Run: func(cmd *cobra.Command, args []string) {
			systemd.Require()
			gears.Clean()
		},
	}
	gearCmd.AddCommand(cleanCmd)

	createCmd := &cobra.Command{
		Use:   "install",
		Short: "Install a docker image as a systemd service",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			systemd.Require()
			gears.InitializeData()

			if len(args) != 2 {
				fail(1, "Valid arguments: <gear_id> <image_name>\n")
			}
			imageId := args[0]
			if imageId == "" {
				fail(1, "Argument 1 must be an image to base the gear on\n")
			}
			gearId, err := gears.NewIdentifier(args[1])
			if err != nil {
				fail(1, "Argument 2 must be a valid gear identifier: %s\n", err.Error())
			}
			run(cmd, func(r jobs.JobResponse) jobs.Job {
				return &jobs.InstallContainerJobRequest{
					r,
					jobs.JobRequest{},
					gearId,
					"",
					imageId,
					&jobs.ExtendedInstallContainerData{},
				}
			})
		},
	}
	gearCmd.AddCommand(createCmd)

	gearCmd.Execute()
}
