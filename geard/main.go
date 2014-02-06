package main

import (
	geard ".."
	//"errors"
	"log"
	"net/http"
	//"strings"
)

var dispatcher = geard.Dispatcher{
	QueueFast:         10,
	QueueSlow:         1,
	Concurrent:        2,
	TrackDuplicateIds: 1000,
}

func main() {
	if err := geard.StartSystemdConnection(); err != nil {
		log.Println("WARNING: No systemd connection available via dbus: ", err)
	}
	if err := geard.VerifyDataPaths(); err != nil {
		log.Fatal(err)
	}
	dispatcher.Start()
	listenHttp()
}

func listenHttp() {
	connect := ":8080"
	log.Printf("Starting HTTP on %s ... ", connect)
	log.Fatal(http.ListenAndServe(connect, geard.NewHttpApiHandler(&dispatcher)))
}
