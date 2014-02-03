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
	dispatcher.Start()
	listenHttp()
}

func listenHttp() {
	connect := ":8080"
	log.Printf("Starting HTTP on %s ... ", connect)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		geard.ServeApi(&dispatcher, w, r)
	})
	log.Fatal(http.ListenAndServe(connect, nil))
}
