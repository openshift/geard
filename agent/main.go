package main

import (
	agent ".."
	//"errors"
	"fmt"
	"log"
	"net/http"
	//"strings"
)

var dispatcher = agent.Dispatcher{
	QueueFast:         10,
	QueueSlow:         1,
	Concurrent:        2,
	TrackDuplicateIds: 1000,
}

func main() {
	dispatcher.Start()
	listenHttp()
}

func listenHttp() {
	fmt.Println("Starting HTTP ... ")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		agent.ServeApi(&dispatcher, w, r)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
