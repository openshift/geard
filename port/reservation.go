package port

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

var ErrAllocationFailed = errors.New("A port could not be allocated.")

func (p Port) PortPathsFor() (base string, path string) {
	root := Device("1").DevicePath()
	prefix := p / portsPerBlock
	base = filepath.Join(root, strconv.FormatUint(uint64(prefix), 10))
	path = filepath.Join(base, strconv.FormatUint(uint64(p), 10))
	return
}

func AtomicReserveExternalPorts(path string, ports, existing PortPairs) (PortPairs, error) {
	reservations, errp := ports.reserve()
	if errp != nil {
		return ports, errp
	}
	unreserve, erru := reservations.reuse(existing)
	if erru != nil {
		return ports, erru
	}

	reserved := make(PortPairs, len(reservations))
	for i := range reservations {
		reserved[i] = reservations[i].PortPair
	}

	if err := reservations.reserve(path); err != nil {
		return ports, err
	}

	if len(unreserve) > 0 {
		log.Printf("ports: Releasing %v", unreserve)
	}
	ReleaseExternalPorts(unreserve) // Ignore errors

	return reserved, nil
}

func ReleaseExternalPorts(ports PortPairs) error {
	var err error
	for i := range ports {
		_, direct := ports[i].External.PortPathsFor()
		path, errl := os.Readlink(direct)
		if errl != nil {
			if !os.IsNotExist(errl) {
				// REPAIR: link can't be checked, may be broken
				log.Printf("ports: Path cannot be checked: %v", errl)
				err = errl
			}
			// the port is no longer reserved (link does not exist)
			continue
		}
		if _, errs := os.Stat(path); errs != nil {
			if os.IsNotExist(errs) {
				// referenced container does not exist, remove the link
				os.Remove(direct)
				continue
			}
			// REPAIR: can't read the referenced container
			err = errs
			continue
		}
		if errr := os.Remove(direct); errr != nil {
			log.Printf("ports: Unable to remove symlink %v", errr)
			err = errr
			// REPAIR: reserved ports may not be properly released
			continue
		}
	}
	return err
}

type portReservation struct {
	PortPair
	reserved  bool
	allocated bool
	exists    bool
}

type portReservations []portReservation

// Reserve any unspecified external ports or return an error
// if no ports are available.
func (p PortPairs) reserve() (portReservations, error) {
	reservation := make(portReservations, len(p))
	for i := range p {
		res := &reservation[i]
		res.PortPair = p[i]
	}
	return reservation, nil
}

// Write reservations to disk or return an error.  Will
// attempt to clean up after a failure by removing partially
// created links.
func (p portReservations) reserve(path string) error {
	var err error
	for i := range p {
		res := &p[i]
		if !res.exists {
			parent, direct := res.External.PortPathsFor()
			os.MkdirAll(parent, 0770)
			err = os.Symlink(path, direct)
			if err != nil {
				log.Printf("ports: Failed to reserve %d, rolling back: %v", res.External, err)
				break
			}
			res.allocated = true
		}
	}

	if err != nil {
		for i := range p {
			res := &p[i]
			if res.allocated {
				_, direct := res.External.PortPathsFor()
				if errr := os.Remove(direct); errr == nil {
					log.Printf("ports: Unable to rollback allocation %d: %v", res.External, err)
					res.allocated = false
				}
			}
		}
		return err
	}

	return nil
}

// Use existing port pairs where possible instead of allocating new ports.
func (p portReservations) reuse(existing PortPairs) (PortPairs, error) {
	unreserve := make(PortPairs, 0, 4)
	for j := range existing {
		ex := &existing[j]
		matched := false
		for i := range p {
			res := &p[i]
			if res.Internal == ex.Internal {
				if res.exists {
					return unreserve, errors.New(fmt.Sprintf("The internal port %d is allocated to more than one external port.", res.Internal))
				}
				if res.External == 0 {
					// Use an already allocated port
					res.External = ex.External
					res.exists = true
				} else if res.External != ex.External {
					unreserve = append(unreserve, PortPair{0, ex.External})
				} else {
					res.exists = true
				}
				if res.exists {
					_, direct := ex.External.PortPathsFor()
					if _, err := os.Stat(direct); err != nil {
						res.External = 0
						res.exists = false
					}
				}
				matched = true
			}
		}
		if !matched {
			unreserve = append(unreserve, *ex)
		}
	}
	for i := range p {
		res := &p[i]
		if res.External == 0 {
			res.External = allocatePort()
			if res.External == 0 {
				return unreserve, ErrAllocationFailed
			}
			res.reserved = true
		}
	}
	return unreserve, nil
}
