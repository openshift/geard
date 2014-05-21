package systemd

import (
	"fmt"
	"github.com/openshift/go-systemd/dbus"
	"os"
	"strings"

	"github.com/openshift/geard/containers"
)

type EventListener struct {
	conn      *dbus.Conn
	exitChan  chan bool
	lastEvent map[containers.Identifier]EventType
}

type EventType int

const (
	Unknown EventType = iota
	Started
	Idled
	Stopped
	Deleted
	Errored
)

type ContainerEvent struct {
	Id   containers.Identifier
	Type EventType
}

func (e ContainerEvent) String() string {
	switch {
	case e.Type == Unknown:
		return string(e.Id) + " (unknown)"
	case e.Type == Started:
		return string(e.Id) + " (started)"
	case e.Type == Idled:
		return string(e.Id) + " (idled)"
	case e.Type == Stopped:
		return string(e.Id) + " (stopped)"
	case e.Type == Deleted:
		return string(e.Id) + " (deleted)"
	case e.Type == Errored:
		return string(e.Id) + " (error)"
	}
	return string(e.Id) + " (unknown)"
}

func NewEventListener() (*EventListener, error) {
	e := EventListener{}
	var err error

	e.conn, err = dbus.New()
	if err != nil {
		return nil, err
	}

	err = e.conn.Subscribe()
	if err != nil {
		return nil, err
	}

	e.lastEvent = make(map[containers.Identifier]EventType)

	return &e, nil
}

func (e *EventListener) runner(errorChan chan error, eventChan chan *ContainerEvent) {
	updateChan := make(chan *dbus.SubStateUpdate, 500)
	e.conn.SetSubStateSubscriber(updateChan, errorChan)
	for true {
		select {
		case update := <-updateChan:
			unit := update.UnitName
			if !strings.HasPrefix(unit, containers.IdentifierPrefix) {
				continue
			}

			id, err := containers.NewIdentifier(unit[len(containers.IdentifierPrefix):(len(unit) - len(".service"))])
			if err != nil {
				select {
				case errorChan <- err:
				}
				continue
			}

			var (
				fileExists bool
				started    bool
				idleFlag   bool
				event      ContainerEvent
			)

			if _, err := os.Stat(id.UnitPathFor()); err == nil {
				fileExists = true
			}
			if _, err := os.Stat(id.IdleUnitPathFor()); err == nil {
				idleFlag = true
			}
			started, _ = UnitStartOnBoot(id)

			if fileExists == false {
				event = ContainerEvent{id, Deleted}
			} else {
				if started {
					if update.ActiveState == "active" {
						event = ContainerEvent{id, Started}
					} else {
						if idleFlag {
							event = ContainerEvent{id, Idled}
						} else {
							if update.ActiveState == "failed" {
								event = ContainerEvent{id, Errored}
							} else {
								event = ContainerEvent{id, Unknown}
							}
						}
					}
				} else {
					event = ContainerEvent{id, Stopped}
				}
			}

			if e.lastEvent[id] == event.Type {
				continue
			} else {
				e.lastEvent[id] = event.Type
			}

			select {
			case eventChan <- (&event):
			}
		case <-e.exitChan:
			return
		}
	}

	fmt.Println("Event listener exited")
}

func (e *EventListener) Run() (<-chan *ContainerEvent, <-chan error) {
	errorChan := make(chan error, 10)
	e.exitChan = make(chan bool)
	eventChan := make(chan *ContainerEvent, 100)

	go e.runner(errorChan, eventChan)
	return eventChan, errorChan
}

func (e *EventListener) Stop() {
	e.exitChan <- true
}

func (e *EventListener) Close() {
	e.conn.Unsubscribe()
}
