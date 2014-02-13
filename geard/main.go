package main

import (
	geard ".."
	"code.google.com/p/go.crypto/ssh"
	"log"
	"net/http"
	"sync"
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
	if err := geard.InitializeSlices(); err != nil {
		log.Fatal(err)
	}
	geard.StartPortAllocator(4000, 60000)
	dispatcher.Start()
	wg := &sync.WaitGroup{}
	//listenSsh(wg)
	listenHttp(wg)
	wg.Wait()
	log.Print("Exiting ...")
}

func listenSsh(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		connect := ":2022"
		server := &geard.SshServer{}
		log.Printf("Starting SSHD on %s ... ", connect)
		log.Fatal(server.ListenAndServe(connect, &ssh.ServerConfig{}))
	}()
}

func listenHttp(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		connect := ":8080"
		log.Printf("Starting HTTP on %s ... ", connect)
		log.Fatal(http.ListenAndServe(connect, geard.NewHttpApiHandler(&dispatcher)))
	}()
}
