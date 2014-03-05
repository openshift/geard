package cmd

import (
	"fmt"
	"github.com/smarterclayton/cobra"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/git"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/systemd"
	"log"
	nethttp "net/http"
	"os"
	"os/user"
	"strconv"
)

var (
	pre        bool
	post       bool
	follow     bool
	listenAddr string
	portPairs  PortPairs
)

var conf = http.HttpConfiguration{
	Dispatcher: &dispatcher.Dispatcher{
		QueueFast:         10,
		QueueSlow:         1,
		Concurrent:        2,
		TrackDuplicateIds: 1000,
	},
	Extensions: []http.HttpExtension{
		git.Routes,
	},
}

// Parse the command line arguments and invoke one of the support subcommands.
func Execute() {
	gearCmd := &cobra.Command{
		Use:   "gear",
		Short: "Gear(d) is a tool for installing Docker containers to systemd",
		Long:  "A commandline client and server that allows Docker containers to be installed to Systemd in an opinionated and distributed fashion.\n\nComplete documentation is available at http://github.com/smarterclayton/geard",
		Run:   gear,
	}
	gearCmd.PersistentFlags().StringVarP(&(conf.Docker.Socket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")

	installImageCmd := &cobra.Command{
		Use:   "install <image> <name>...",
		Short: "Install a docker image as a systemd service",
		Long:  "Given a docker image label (which may include a custom registry) and the name of one or more gears, contact each of the requested servers and install the image as a new container managed by systemd.\n\nSpecify a location on a remote server with <host>[:<port>]/<name> instead of <name>.  The default port is 2223.",
		Run:   installImage,
	}
	installImageCmd.Flags().VarP(&portPairs, "ports", "p", "List of comma separated port pairs to bind '<internal>=<external>,...'.\nUse zero to request a port be assigned.")
	gearCmd.AddCommand(installImageCmd)

	startCmd := &cobra.Command{
		Use:   "start <name>...",
		Short: "Invoke systemd to start a gear",
		Long:  "Queues the start and immediately returns.", //  Use -f to attach to the logs.",
		Run:   startContainer,
	}
	//startCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Attach to the logs after startup")
	gearCmd.AddCommand(startCmd)

	stopCmd := &cobra.Command{
		Use:   "stop <name>...",
		Short: "Invoke systemd to stop a gear",
		Long:  ``,
		Run:   stopContainer,
	}
	gearCmd.AddCommand(stopCmd)

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear server",
		Long:  "Launch the gear HTTP API server as a daemon. Will not send itself to the background.",
		Run:   daemon,
	}
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":8080", "Set the address for the http endpoint to listen on")
	gearCmd.AddCommand(daemonCmd)

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "(Local) Disable all gears, slices, and targets in systemd",
		Long:  "Disable all registered resources from systemd to allow them to be removed from the system.  Will reload the systemd daemon config.",
		Run:   clean,
	}
	gearCmd.AddCommand(cleanCmd)

	initGearCmd := &cobra.Command{
		Use:   "init <name> <image>",
		Short: "(Local) Setup the environment for a gear",
		Long:  "",
		Run:   initGear,
	}
	initGearCmd.Flags().BoolVarP(&pre, "pre", "", false, "Perform pre-start initialization")
	initGearCmd.Flags().BoolVarP(&post, "post", "", false, "Perform post-start initialization")
	gearCmd.AddCommand(initGearCmd)

	genAuthKeysCmd := &cobra.Command{
		Use:   "gen-auth-keys [<name>]",
		Short: "(Local) Create the authorized_keys file for a gear",
		Long:  "Generate .ssh/authorized_keys file for the specified gear id or (if gear id is ommitted) for the current gear user",
		Run:   genAuthKeys,
	}
	gearCmd.AddCommand(genAuthKeysCmd)

	gearCmd.Execute()
}

// Initializers for local command execution.
func needsSystemd() {
	systemd.Require()
}
func needsSystemdAndData() {
	systemd.Require()
	containers.InitializeData()
}

func gear(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func daemon(cmd *cobra.Command, args []string) {
	systemd.Start()
	containers.InitializeData()
	containers.StartPortAllocator(4000, 60000)
	conf.Dispatcher.Start()

	nethttp.Handle("/", conf.Handler())
	log.Printf("Listening for HTTP on %s ...", listenAddr)
	log.Fatal(nethttp.ListenAndServe(listenAddr, nil))
}

func clean(cmd *cobra.Command, args []string) {
	needsSystemd()
	containers.Clean()
}

func installImage(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fail(1, "Valid arguments: <image_name> <id> ...\n")
	}
	imageId := args[0]
	if imageId == "" {
		fail(1, "Argument 1 must be an image to base the gear on\n")
	}
	ids, err := NewRemoteIdentifiers(args[1:])
	if err != nil {
		fail(1, "You must pass one or more valid gear ids: %s\n", err.Error())
	}

	runEach(cmd, needsSystemdAndData, func(on Locator) jobs.Job {
		return &http.HttpInstallContainerRequest{
			InstallContainerRequest: jobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),
				Id:                on.(*RemoteIdentifier).Id,
				Image:             imageId,
				Ports:             *portPairs.Get().(*containers.PortPairs),
			},
		}
	}, ids...)
}

func startContainer(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args)
	if err != nil {
		fail(1, "You must pass one or more valid gear ids: %s\n", err.Error())
	}

	fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl start %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	runEach(cmd, needsSystemd, func(on Locator) jobs.Job {
		return &http.HttpStartContainerRequest{
			StartedContainerStateRequest: jobs.StartedContainerStateRequest{
				Id: on.(*RemoteIdentifier).Id,
			},
		}
	}, ids...)
}

func stopContainer(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args)
	if err != nil {
		fail(1, "You must pass one or more valid gear ids: %s\n", err.Error())
	}

	fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl stop %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	runEach(cmd, needsSystemd, func(on Locator) jobs.Job {
		return &http.HttpStopContainerRequest{
			StoppedContainerStateRequest: jobs.StoppedContainerStateRequest{
				Id: on.(*RemoteIdentifier).Id,
			},
		}
	}, ids...)
}

func initGear(cmd *cobra.Command, args []string) {
	if len(args) != 2 || !(pre || post) || (pre && post) {
		fail(1, "Valid arguments: <gear_id> <image_name> (--pre|--post)\n")
	}
	gearId, err := containers.NewIdentifier(args[0])
	if err != nil {
		fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
	}

	switch {
	case pre:
		if err := containers.InitPreStart(conf.Docker.Socket, gearId, args[1]); err != nil {
			fail(2, "Unable to initialize container %s\n", err.Error())
		}
	case post:
		if err := containers.InitPostStart(conf.Docker.Socket, gearId); err != nil {
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

	if len(args) == 1 {
		gearId, err := containers.NewIdentifier(args[0])
		if err != nil {
			fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
		}
		if u, err = user.Lookup(gearId.LoginFor()); err != nil {
			fail(2, "Unable to lookup user: %s", err.Error())
		}
	} else {
		if u, err = user.LookupId(strconv.Itoa(os.Getuid())); err != nil {
			fail(2, "Unable to lookup user")
		}
	}

	if err := containers.GenerateAuthorizedKeys(conf.Docker.Socket, u); err != nil {
		fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
	}
}
