package geard

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type Port uint

type PortPair struct {
	Internal Port
	External Port
}

type device string

func (d device) DevicePath() string {
	return filepath.Join(GearBasePath(), "ports", "interfaces", string(d))
}

func (p Port) PortPathsFor() (base string, path string) {
	root := device("1").DevicePath()
	prefix := p / portsPerBlock
	base = filepath.Join(root, strconv.FormatUint(uint64(prefix), 10))
	path = filepath.Join(base, strconv.FormatUint(uint64(p), 10))
	return
}

func AtomicReserveExternalPorts(path string, pairs []PortPair) error {
	var contents bytes.Buffer
	for _, pair := range pairs {
		if pair.External == 0 {
			return errors.New("External port must be non-zero")
		}
		contents.WriteString(fmt.Sprintf("%d\t%d\n", pair.Internal, pair.External))
	}

	if erra := AtomicWriteToContentPath(path, 0770, contents.Bytes()); erra != nil {
		log.Print("ports: Unable to write port description: ", erra)
		return erra
	}

	for _, ports := range pairs {
		parent, direct := Port(ports.External).PortPathsFor()
		os.MkdirAll(parent, 0770)
		if errs := os.Symlink(path, direct); errs != nil {
			log.Print("ports: Unable to reserve port on disk: ", errs)
			return errs
		}
	}

	return nil
}

const portsPerBlock = Port(100) // changing this breaks disk structure... don't do it!
const maxReadFailures = 3

//
// Returns 0 if no port can be allocated.  Consumers
// should fail when getting 0 - more ports may become
// available at a later time, but are unlikely to
// come open now.
//
func AllocatePort() Port {
	p := <-internalPortAllocator.ports
	log.Printf("ports: Reserved port %d", p)
	return p
}

func StartPortAllocator(min, max Port) {
	internalPortAllocator.min = min
	internalPortAllocator.max = max
	internalPortAllocator.block = uint(min / portsPerBlock)
	go func() {
		internalPortAllocator.findPorts()
		close(internalPortAllocator.ports)
	}()
}

//
// An example of a very simple Port allocator.
//
type portAllocator struct {
	ports    chan Port
	done     chan bool
	block    uint
	failures int
	min      Port
	max      Port
}

var internalPortAllocator = portAllocator{make(chan Port), make(chan bool), 1, 0, 0, 0}

func (p *portAllocator) findPorts() {
	for {
		foundInBlock := 0
		start := Port(p.block) * portsPerBlock
		if start < p.min {
			start = p.min
		}
		end := (Port(p.block) + 1) * portsPerBlock
		if end > p.max {
			end = p.max
			p.block = uint(p.min / portsPerBlock)
		} else {
			p.block += 1
		}
		log.Printf("ports: searching block %d, %d-%d", p.block, start, end-1)

		var taken []string
		parent, _ := start.PortPathsFor()
		f, erro := os.OpenFile(parent, os.O_RDONLY, 0)
		if erro == nil {
			names, errr := f.Readdirnames(int(portsPerBlock))
			f.Close()
			if errr != nil {
				log.Printf("ports: failed to read %s: %v", parent, errr)
				if p.fail() {
					goto finished
				}
				continue
			}
			taken = names
		}

		if reserved := namesToPorts(taken); len(reserved) > 0 {
			existing := reserved[0]
			other := 1
			for n := start; n < end; n++ {
				if existing == n {
					if other < len(reserved) {
						existing = reserved[other]
						other += 1
					}
					continue
				}
				select {
				case p.ports <- n:
					foundInBlock += 1
				case <-p.done:
					goto finished
				}
			}
		} else {
			for n := start; n < end; n++ {
				select {
				case p.ports <- n:
					foundInBlock += 1
				case <-p.done:
					goto finished
				}
			}
		}

		if foundInBlock == 0 {
			log.Printf("ports: failed to find a port between %d-%d ", start, end-1)
			if p.fail() {
				goto finished
			}
		} else {
			p.foundPorts()
		}
	}
finished:
}

func (p *portAllocator) fail() bool {
	p.failures += 1
	if p.failures > maxReadFailures {
		select {
		case p.ports <- 0:
		case <-p.done:
			return true
		}
	}
	return false
}

func (p *portAllocator) foundPorts() {
	p.failures = 0
}

type ports []Port

func (a ports) Len() int           { return len(a) }
func (a ports) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ports) Less(i, j int) bool { return a[i] < a[j] }

func namesToPorts(reservedNames []string) ports {
	if len(reservedNames) == 0 {
		return ports{}
	}
	reserved := make(ports, len(reservedNames))
	converted := false
	for i := range reservedNames {
		if v, err := strconv.Atoi(reservedNames[i]); err == nil {
			converted = true
			reserved[i] = Port(v)
		}
	}
	if converted {
		sort.Sort(reserved)
		for i := 0; i < len(reserved); i++ {
			if reserved[i] != 0 {
				return reserved[i:]
			}
		}
	}
	return ports{}
}
