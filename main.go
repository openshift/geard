package main

import (
	"flag"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/gear"
	"github.com/smarterclayton/geard/http"

	"log"
	_ "net/http/pprof"
	"os"
	"sync"
)

var dispatch = dispatcher.Dispatcher{
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
	dispatch.Start()
	wg := &sync.WaitGroup{}
	http.StartAPI(wg, &dispatch)
	wg.Wait()
	log.Print("Exiting ...")
}
