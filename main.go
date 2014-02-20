package main

import (
	"flag"
	"github.com/smarterclayton/geard/api"
	"github.com/smarterclayton/geard/gear"
	"github.com/smarterclayton/geard/dispatcher"

	"log"
	_ "net/http/pprof"
	"os"
	"sync"
)

var dpatcher = dispatcher.Dispatcher{
	QueueFast:         10,
	QueueSlow:         1,
	Concurrent:        2,
	TrackDuplicateIds: 1000,
}

func main() {
	var clean bool
	flag.BoolVar(&clean, "clean", false, "Reset the state of the system and unregister gears")

	flag.Parse()

	Initialize()
	if clean {
		Clean()
		os.Exit(1)
	}

	gear.StartPortAllocator(4000, 60000)
	dpatcher.Start()
	wg := &sync.WaitGroup{}
	api.StartAPI(wg, &dpatcher)
	wg.Wait()
	log.Print("Exiting ...")
}
