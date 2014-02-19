package main

import (
	"flag"
	"github.com/smarterclayton/geard"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
)

var dispatcher = geard.Dispatcher{
	QueueFast:         10,
	QueueSlow:         1,
	Concurrent:        2,
	TrackDuplicateIds: 1000,
}

func init() {
	var clean bool
	flag.BoolVar(&clean, "clean", false, "Reset the state of the system and unregister gears")

	flag.Parse()

	if err := geard.StartSystemdConnection(); err != nil {
		log.Println("WARNING: No systemd connection available via dbus: ", err)
		log.Println("  You may need to run as root or check that /var/run/dbus/system_bus_socket is bind mounted.")
	}

	if clean {
		geard.DisableAllUnits()
		os.Exit(1)
	}
}

func main() {
	if err := geard.VerifyDataPaths(); err != nil {
		log.Fatal(err)
	}
	if err := geard.InitializeTargets(); err != nil {
		log.Fatal(err)
	}
	if err := geard.InitializeSlices(); err != nil {
		log.Fatal(err)
	}
	if err := geard.InitializeBinaries(); err != nil {
		log.Fatal(err)
	}
	geard.StartPortAllocator(4000, 60000)
	dispatcher.Start()
	wg := &sync.WaitGroup{}
	listenHttp(wg)
	wg.Wait()
	log.Print("Exiting ...")
}

func listenHttp(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		connect := ":8080"
		log.Printf("Starting HTTP on %s ... ", connect)
		http.Handle("/", geard.NewHttpApiHandler(&dispatcher))
		log.Fatal(http.ListenAndServe(connect, nil))
	}()
}
