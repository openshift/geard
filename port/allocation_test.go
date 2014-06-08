package port

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAllocatePorts(t *testing.T) {
	dir, _ := ioutil.TempDir(os.TempDir(), "porttest")
	defer os.RemoveAll(dir)
	alloc := NewPortAllocator(dir, 40000, 40010)

	base, path := alloc.portPathsFor(Port(40001))
	os.MkdirAll(base, 0700)
	os.Create(path)

	go alloc.Run()
	if p := alloc.allocatePort(); p != 40000 {
		t.Errorf("Expected to allocate the first available port, got %d", p)
	}
	if p := alloc.allocatePort(); p != 40002 {
		t.Errorf("Expected to skip a port reserved on disk, got %d", p)
	}
	if p := alloc.allocatePort(); p != 40003 {
		t.Errorf("Should allocate next port %d", p)
	}
}

func TestReserveReleasePorts(t *testing.T) {
	dir, _ := ioutil.TempDir(os.TempDir(), "porttest")
	defer os.RemoveAll(dir)
	alloc := NewPortAllocator(dir, 40000, 40010)

	base, link := alloc.portPathsFor(Port(40001))
	os.MkdirAll(base, 0700)
	os.Create(link)

	path := filepath.Join(dir, "unit1")
	if _, err := os.Create(path); err != nil {
		t.Errorf("Couldn't create temporary file: %s", err.Error())
	}

	go alloc.Run()
	reserve := PortReservation{alloc}

	p, err := reserve.AtomicReserveExternalPorts(path, PortPairs{PortPair{8080, 0}}, PortPairs{})
	if err != nil {
		t.Errorf("Couldn't reserve ports: %s", err.Error())
	}
	if len(p) != 1 && p[0].External != 8080 && p[0].Internal != 40000 {
		t.Errorf("Did not reserve expected port, %+v", p)
	}
	_, expected := alloc.portPathsFor(Port(40000))
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("Unable to find %s: %s", expected, err.Error())
	}
	if s, _ := os.Readlink(expected); s != path {
		t.Errorf("Reservation did not link to the supplied file, %s -> %s (actual: %s)", expected, path, s)
	}

	if err := reserve.ReleaseExternalPorts(p); err != nil {
		t.Errorf("Did not release expected port, %+v", p)
	}
	if _, err := os.Stat(expected); !os.IsNotExist(err) {
		t.Errorf("Should have removed link on filesystem %s: %+v", expected, err)
	}
}
