package main

import (
	"github.com/smarterclayton/cobra"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/http"
	"log"
	"os"
	"sync"
)

func Execute() {
	daemon := false
	gearCmd := &cobra.Command{
		Use:   "gear",
		Short: "Gear(d) is a tool for installing Docker containers to systemd",
		Long: `A commandline client and server that allows Docker containers to
              be installed to Systemd in an opinionated and distributed
              fashion.
              Complete documentation is available at http://github.com/smarterclayton/geard`,
		Run: func(cmd *cobra.Command, args []string) {
			if daemon {
				InitializeSystemd()
				InitializeData()
				var dispatch = dispatcher.Dispatcher{
					QueueFast:         10,
					QueueSlow:         1,
					Concurrent:        2,
					TrackDuplicateIds: 1000,
				}
				dispatch.Start()
				gears.StartPortAllocator(4000, 60000)
				wg := &sync.WaitGroup{}
				http.StartAPI(wg, &dispatch)
				wg.Wait()
				log.Print("Exiting ...")
				return
			}

			cmd.Usage()
		},
	}
	gearCmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run as a server process")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Disable all gears, slices, and targets in systemd",
		Long: `Disable all registered resources from systemd to allow them to be
              removed from the system.  Will reload the systemd daemon config.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := InitializeSystemd(); err != nil {
				log.Print("Systemd connection is required for cleanup.")
				os.Exit(1)
			}
			Clean()
		},
	}
	gearCmd.AddCommand(cleanCmd)

	gearCmd.Execute()
}
