// +build idler

package idler

import (
	"github.com/openshift/geard/pkg/go-netfilter-queue"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/idler/config"
	"github.com/openshift/geard/idler/iptables"
	"github.com/openshift/geard/systemd"

	"bytes"
	"code.google.com/p/gopacket/layers"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

type Idler struct {
	d             *docker.DockerClient
	qh            []*netfilter.NFQueue
	waitChan      chan uint16
	openChannels  []containers.Identifier
	hostIp        string
	idleTimeout   time.Duration
	eventListener *containers.EventListener
}

var idler *Idler

func StartIdler(pDockerClient *docker.DockerClient, pHostIp string, pIdleTimeout int) error {
	idler = newIdler(pDockerClient, pHostIp, pIdleTimeout)
	idler.Run()
	return nil
}

func newIdler(d *docker.DockerClient, hostIp string, idleTimeout int) *Idler {
	var err error

	idler := Idler{}
	idler.d = d
	idler.qh = make([]*netfilter.NFQueue, config.NumQueues)
	idler.waitChan = make(chan uint16)
	idler.openChannels = make([]containers.Identifier, config.NumQueues)
	idler.hostIp = hostIp
	idler.eventListener, err = containers.NewEventListener()
	if err != nil {
		fmt.Printf("Unable to create SystemD event listener: %v\n", err)
		return nil
	}
	for i := 0; i < config.NumQueues; i++ {
		idler.qh[i], err = netfilter.NewNFQueue(uint16(i), 100, netfilter.NF_DEFAULT_PACKET_SIZE)
		if err != nil {
			fmt.Printf("Unable to open Netfilter Queue: %v\n", err)
			return nil
		}
	}
	idler.idleTimeout = time.Minute * time.Duration(idleTimeout)

	return &idler
}

func (idler *Idler) Run() {
	for i := range idler.qh {
		if i >= 1 {
			go waitStart(idler.qh[i].GetPackets(), uint16(i), idler.waitChan, idler.hostIp)
		}
	}

	packets := idler.qh[0].GetPackets()
	ticker := time.NewTicker(idler.idleTimeout)
	events, errors := idler.eventListener.Run()

	for true {
		select {
		case e := <-events:
			fmt.Printf("[%v] Event: %v\n", time.Now().Format(time.RFC3339), e)
			switch {
			case e.Type == containers.Stopped || e.Type == containers.Deleted || e.Type == containers.Errored:
				iptables.DeleteContainer(e.Id, idler.hostIp)
			case e.Type == containers.Started:
				iptables.UnidleContainer(e.Id, idler.hostIp)
			case e.Type == containers.Idled:
				//No-op
			}
		case e := <-errors:
			fmt.Printf("Error: %v\n", e)
		case chanId := <-idler.waitChan:
			idler.openChannels[chanId] = ""
		case p := <-packets:
			port, err := portForPacket(p)
			if err != nil {
				fmt.Println(err)
				p.SetVerdict(netfilter.NF_ACCEPT)
				continue
			}

			id, err := port.IdentifierFor()
			if err != nil {
				fmt.Println(err)
				iptables.CleanupRulesForPort(port)
				p.SetVerdict(netfilter.NF_ACCEPT)
				continue
			}

			idler.unidleContainer(id, p)
		case <-ticker.C:
			cpkt, err := iptables.GetDockerContainerPacketCounts(idler.d)
			if err != nil {
				fmt.Printf("Error retrieving packet counts for containers: %v\n", err)
			}

			var packetData bytes.Buffer
			w := new(tabwriter.Writer)
			w.Init(&packetData, 0, 8, 0, '\t', 0)

			fmt.Fprintf(w, "[%v] Packet counts:\n\tContainer\tActive?\tIdled?\tPackets\n", time.Now().Format(time.RFC3339))
			iptables.ResetPacketCount()
			for id, pkts := range cpkt {
				started, err := id.UnitStartOnBoot()
				if err != nil {
					fmt.Printf("Error reading container state for %v: %v\n", id, err)
				}

				var idleFlag bool
				_, err = os.Stat(id.IdleUnitPathFor())
				if err == nil {
					idleFlag = true
				}

				fmt.Fprintf(w, "\t%v\t%v\t%v\t%v", id, started, idleFlag, pkts)
				if started && pkts == 0 {
					if idler.idleContainer(id) {
						fmt.Fprintf(w, "\tidling...")
					}
				}
				fmt.Fprintf(w, "\n")
			}
			w.Flush()
			packetData.WriteTo(os.Stdout)
			fmt.Println("\n")
		}
	}
}

func (idler *Idler) unidleContainer(id containers.Identifier, p netfilter.NFPacket) {
	newChanId, wasAlreadyAssigned := idler.getAvailableWaiter(id)

	if newChanId == 0 {
		fmt.Println("unidle: Error while finding wait channel")
		return
	}

	if !wasAlreadyAssigned {
		//TODO: Ask geard to unidle container
		if err := os.Remove(id.IdleUnitPathFor()); err != nil {
			fmt.Printf("unidle: Could not remove idle marker for %s: %v", id.UnitNameFor(), err)
			p.SetVerdict(netfilter.NF_ACCEPT)
			return
		}
		if err := systemd.Connection().StartUnitJob(id.UnitNameFor(), "fail"); err != nil {
			fmt.Printf("unidle: Could not start container %s: %v", id.UnitNameFor(), err)
			p.SetVerdict(netfilter.NF_ACCEPT)
			return
		}
	}

	p.SetRequeueVerdict(newChanId)
}

func (idler *Idler) getAvailableWaiter(id containers.Identifier) (uint16, bool) {
	for true {
		//existing queue is already processing id
		for i := range idler.openChannels {
			if i != 0 && idler.openChannels[i] == id {
				return uint16(i), true
			}
		}

		for i := range idler.openChannels {
			if i != 0 && (idler.openChannels[i] == "") {
				idler.openChannels[i] = id
				return uint16(i), false
			}
		}

		//Wait for channels to open
		time.Sleep(time.Second)
	}
	return 0, false
}

func (idler *Idler) idleContainer(id containers.Identifier) bool {
	portPairs, err := containers.GetExistingPorts(id)
	if err != nil {
		fmt.Printf("idler.idleContainer: Error retrieving ports for container: %v\n", id)
		return false
	}

	iptablePorts, err := iptables.GetIdlerRules(id, false)
	if err != nil {
		fmt.Printf("idler.idleContainer: Error retrieving ports from iptables: %v\n", id)
		return false
	}

	shouldRecreateRules := false
	for _, portPair := range portPairs {
		extPort := strconv.Itoa(int(portPair.External))
		shouldRecreateRules = shouldRecreateRules || !iptablePorts[extPort]
	}

	if !shouldRecreateRules {
		return false
	}

	//TODO: Ask geard to idle container
	f, err := os.Create(id.IdleUnitPathFor())
	if err != nil {
		fmt.Printf("idler.idleContainer: Could not create idle marker for %s: %v", id.UnitNameFor(), err)
		return false
	}
	f.Close()
	if err := systemd.Connection().StopUnitJob(id.UnitNameFor(), "fail"); err != nil {
		fmt.Printf("idler.idleContainer: Could not stop container %s: %v", id.UnitNameFor(), err)
		return false
	}

	iptables.IdleContainer(id, idler.hostIp)
	return true
}

func waitStart(pChan <-chan netfilter.NFPacket, chanId uint16, waitChan chan<- uint16, hostIp string) {
	for true {
		p := <-pChan

		port, err := portForPacket(p)
		if err != nil {
			fmt.Println(err)
			p.SetVerdict(netfilter.NF_ACCEPT)
			waitChan <- chanId
			continue
		}

		id, err := port.IdentifierFor()
		if err != nil {
			fmt.Println(err)
			p.SetVerdict(netfilter.NF_ACCEPT)
			waitChan <- chanId
			continue
		}

		cInfo, err := systemd.Connection().GetUnitProperties(id.UnitNameFor())
		if err != nil || cInfo["ActiveState"] != "active" {
			//TODO: Placeholder for container start detection
			fmt.Printf("[%v] Waiting for container %v to start\n", time.Now().Format(time.RFC3339), id)
			time.Sleep(time.Second * 5)
			fmt.Printf("[%v] Container %v started\n", time.Now().Format(time.RFC3339), id)

			iptables.UnidleContainer(id, hostIp)
		}

		p.SetVerdict(netfilter.NF_ACCEPT)
		waitChan <- chanId
	}
}

func portForPacket(p netfilter.NFPacket) (containers.Port, error) {
	tcpLayer := p.Packet.TransportLayer()
	tcp, ok := tcpLayer.(*layers.TCP)
	if !ok {
		return 0, fmt.Errorf("Unknown packet of type %v\n", tcpLayer.LayerType())
	}

	return containers.Port(tcp.DstPort), nil
}
