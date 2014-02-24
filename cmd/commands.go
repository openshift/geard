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
	"os/user"	
	"strings"
	"sync"
	"strconv"
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

var (
	daemon bool
	conf   http.HttpConfiguration
)

func Execute() {
	gearCmd := &cobra.Command{
		Use:   "gear",
		Short: "Gear(d) is a tool for installing Docker containers to systemd",
		Long: `A commandline client and server that allows Docker containers to
              be installed to Systemd in an opinionated and distributed
              fashion.
              Complete documentation is available at http://github.com/smarterclayton/geard`,
		Run: runServer,
	}
	gearCmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run as a server process")
	gearCmd.Flags().StringVarP(&(conf.DockerSocket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")
	gearCmd.Flags().StringVarP(&(conf.ListenAddr), "listen-address", "A", ":8080", "Set the address for the http endpoint to listen on")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Disable all gears, slices, and targets in systemd",
		Long: `Disable all registered resources from systemd to allow them to be
              removed from the system.  Will reload the systemd daemon config.`,
		Run: clean,
	}
	gearCmd.AddCommand(cleanCmd)

	createCmd := &cobra.Command{
		Use:   "install",
		Short: "Install a docker image as a systemd service",
		Long:  ``,
		Run:   installImage,
	}
	gearCmd.AddCommand(createCmd)

	initGearCmd := &cobra.Command{
		Use:   "init",
		Short: `Setup the environment for a gear`,
		Long:  ``,
		Run:   gearInit,
	}
	initGearCmd.Flags().Bool("pre", false, "Perform pre-start initialization")
	initGearCmd.Flags().Bool("post", false, "Perform post-start initialization")
	gearCmd.AddCommand(initGearCmd)
	
	genAuthKeysCmd := &cobra.Command{
		Use:   "gen-auth-keys",
		Short: `Generate .ssh/authorized_keys file for the specified gear id or (if gear id is ommitted) for the current gear user`,
		Long:  ``,
		Run:   genAuthKeys,
	}
	gearCmd.AddCommand(genAuthKeysCmd)

	gearCmd.Execute()
}

func runServer(cmd *cobra.Command, args []string) {
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
}

func clean(cmd *cobra.Command, args []string) {
	systemd.Require()
	gears.Clean()
}

func installImage(cmd *cobra.Command, args []string) {
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
}

func gearInit(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	fPre := flags.Lookup("pre").Value.String() == "true"
	fPost := flags.Lookup("post").Value.String() == "true"

	if len(args) != 2 || !(fPre || fPost) || (fPre && fPost) {
		fail(1, "Valid arguments: <gear_id> <image_name> (--pre|--post)\n")
	}
	gearId, err := gears.NewIdentifier(args[0])
	if err != nil {
		fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
	}

	switch{
	case fPre:
		if err := gears.InitPreStart(conf.DockerSocket, gearId, args[1]); err != nil{
			fail(2, "Unable to initialize container %s\n", err.Error())
		}
	case fPost:
		if err := gears.InitPostStart(conf.DockerSocket, gearId); err != nil{
			fail(2, "Unable to initialize container %s\n", err.Error())
		}
	}
}

func genAuthKeys(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fail(1, "Valid arguments: [<gear_id>]\n")
	}
	
	var u *user.User
	var err error
	
	if(len(args) == 1){
		gearId, err := gears.NewIdentifier(args[0])
		if err != nil {
			fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
		}
		if u, err = user.Lookup(gearId.LoginFor()); err != nil {
			fail(2, "Unable to lookup user: %s", err.Error())
		}		
	}else{
		if u, err = user.LookupId(strconv.Itoa(os.Getuid())); err != nil {
			fail(2, "Unable to lookup user")
		}
	}
	
	if err := gears.GenerateAuthorizedKeys(conf.DockerSocket, u); err != nil{
		fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
	}
}
