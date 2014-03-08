// +build selinux

package library

/*
 The selinux package is a go bindings to libselinux required to add selinux
 support to docker.

 Author Dan Walsh <dwalsh@redhat.com>

 Used some ideas/code from the go-ini packages https://github.com/vaughan0
 By Vaughan Newton
*/

// #cgo pkg-config: libselinux
// #include <selinux/selinux.h>
// #include <stdlib.h>
import "C"
import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unsafe"
)

var (
	assignRegex = regexp.MustCompile(`^([^=]+)=(.*)$`)
	mcsList     = make(map[string]bool)
)

func Matchpathcon(path string, mode os.FileMode) (string, error) {
	var con C.security_context_t
	var scon string
	rc, err := C.matchpathcon(C.CString(path), C.mode_t(mode), &con)
	if rc == 0 {
		scon = C.GoString(con)
		C.free(unsafe.Pointer(con))
	}
	return scon, err
}

func Setfilecon(path, scon string) (int, error) {
	rc, err := C.lsetfilecon(C.CString(path), C.CString(scon))
	return int(rc), err
}

func Getfilecon(path string) (string, error) {
	var scon C.security_context_t
	var fcon string
	rc, err := C.lgetfilecon(C.CString(path), &scon)
	if rc >= 0 {
		fcon = C.GoString(scon)
		err = nil
	}
	return fcon, err
}

func Setfscreatecon(scon string) (int, error) {
	var (
		rc  C.int
		err error
	)
	if scon != "" {
		rc, err = C.setfscreatecon(C.CString(scon))
	} else {
		rc, err = C.setfscreatecon(nil)
	}
	return int(rc), err
}

func Getfscreatecon() (string, error) {
	var scon C.security_context_t
	var fcon string
	rc, err := C.getfscreatecon(&scon)
	if rc >= 0 {
		fcon = C.GoString(scon)
		err = nil
		C.freecon(scon)
	}
	return fcon, err
}

func Getcon() string {
	var pcon C.security_context_t
	C.getcon(&pcon)
	scon := C.GoString(pcon)
	C.freecon(pcon)
	return scon
}

func Getpidcon(pid int) (string, error) {
	var pcon C.security_context_t
	var scon string
	rc, err := C.getpidcon(C.pid_t(pid), &pcon)
	if rc >= 0 {
		scon = C.GoString(pcon)
		C.freecon(pcon)
		err = nil
	}
	return scon, err
}

func Getpeercon(socket int) (string, error) {
	var pcon C.security_context_t
	var scon string
	rc, err := C.getpeercon(C.int(socket), &pcon)
	if rc >= 0 {
		scon = C.GoString(pcon)
		C.freecon(pcon)
		err = nil
	}
	return scon, err
}

func Setexeccon(scon string) error {
	var val *C.char
	if !SelinuxEnabled() {
		return nil
	}
	if scon != "" {
		val = C.CString(scon)
	} else {
		val = nil
	}
	_, err := C.setexeccon(val)
	return err
}

type Context struct {
	con []string
}

func (c *Context) SetUser(user string) {
	c.con[0] = user
}
func (c *Context) GetUser() string {
	return c.con[0]
}
func (c *Context) SetRole(role string) {
	c.con[1] = role
}
func (c *Context) GetRole() string {
	return c.con[1]
}
func (c *Context) SetType(setype string) {
	c.con[2] = setype
}
func (c *Context) GetType() string {
	return c.con[2]
}
func (c *Context) SetLevel(mls string) {
	c.con[3] = mls
}
func (c *Context) GetLevel() string {
	return c.con[3]
}
func (c *Context) Get() string {
	return strings.Join(c.con, ":")
}
func (c *Context) Set(scon string) {
	c.con = strings.SplitN(scon, ":", 4)
}
func NewContext(scon string) Context {
	var con Context
	con.Set(scon)
	return con
}

func SelinuxEnabled() bool {
	b := C.is_selinux_enabled()
	if b > 0 {
		return true
	}
	return false
}

const (
	Enforcing  = 1
	Permissive = 0
	Disabled   = -1
)

func SelinuxGetEnforce() int {
	return int(C.security_getenforce())
}

func SelinuxGetEnforceMode() int {
	var enforce C.int
	C.selinux_getenforcemode(&enforce)
	return int(enforce)
}

func mcsAdd(mcs string) {
	mcsList[mcs] = true
}

func mcsDelete(mcs string) {
	mcsList[mcs] = false
}

func mcsExists(mcs string) bool {
	return mcsList[mcs]
}

func IntToMcs(id int, catRange uint32) string {
	if (id < 1) || (id > 523776) {
		return ""
	}

	SETSIZE := int(catRange)
	TIER := SETSIZE

	ORD := id
	for ORD > TIER {
		ORD = ORD - TIER
		TIER -= 1
	}
	TIER = SETSIZE - TIER
	ORD = ORD + TIER
	return fmt.Sprintf("s0:c%d,c%d", TIER, ORD)
}

func uniqMcs(catRange uint32) string {
	var n uint32
	var c1, c2 uint32
	var mcs string
	for {
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c1 = n % catRange
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c2 = n % catRange
		if c1 == c2 {
			continue
		} else {
			if c1 > c2 {
				t := c1
				c1 = c2
				c2 = t
			}
		}
		mcs = fmt.Sprintf("s0:c%d,c%d", c1, c2)
		if mcsExists(mcs) {
			continue
		}
		mcsAdd(mcs)
		break
	}
	return mcs
}
func freeContext(processLabel string) {
	var scon Context
	scon = NewContext(processLabel)
	mcsDelete(scon.GetLevel())
}

func GetLxcContexts() (processLabel string, fileLabel string) {
	var val, key string
	var bufin *bufio.Reader
	if !SelinuxEnabled() {
		return
	}
	lxcPath := C.GoString(C.selinux_lxc_contexts_path())
	fileLabel = "system_u:object_r:svirt_sandbox_file_t:s0"
	processLabel = "system_u:system_r:svirt_lxc_net_t:s0"

	in, err := os.Open(lxcPath)
	if err != nil {
		goto exit
	}

	defer in.Close()
	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err == io.EOF {
				done = true
			} else {
				goto exit
			}
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		if groups := assignRegex.FindStringSubmatch(line); groups != nil {
			key, val = strings.TrimSpace(groups[1]), strings.TrimSpace(groups[2])
			if key == "process" {
				processLabel = strings.Trim(val, "\"")
			}
			if key == "file" {
				fileLabel = strings.Trim(val, "\"")
			}
		}
	}
exit:
	var scon Context
	mcs := IntToMcs(os.Getpid(), 1024)
	scon = NewContext(processLabel)
	scon.SetLevel(mcs)
	processLabel = scon.Get()
	scon = NewContext(fileLabel)
	scon.SetLevel(mcs)
	fileLabel = scon.Get()
	return processLabel, fileLabel
}

func CopyLevel(src, dest string) (string, error) {
	if !SelinuxEnabled() {
		return "", nil
	}
	if src == "" {
		return "", nil
	}
	rc, err := C.security_check_context(C.CString(src))
	if rc != 0 {
		return "", err
	}
	rc, err = C.security_check_context(C.CString(dest))
	if rc != 0 {
		return "", err
	}
	scon := NewContext(src)
	tcon := NewContext(dest)
	tcon.SetLevel(scon.GetLevel())
	return tcon.Get(), nil
}

func RestoreCon(fpath string, recurse bool) error {
	var flabel string
	var err error
	var fs os.FileInfo

	if !SelinuxEnabled() {
		return nil
	}

	if recurse {
		var paths []string
		var err error

		if paths, err = filepath.Glob(path.Join(fpath, "**", "*")); err != nil {
			return fmt.Errorf("Unable to find directory %v: %v", fpath, err)
		}

		for _, fpath := range paths {
			if err = RestoreCon(fpath, false); err != nil {
				return fmt.Errorf("Unable to restore selinux context for %v: %v", fpath, err)
			}
		}
		return nil
	}
	if fs, err = os.Stat(fpath); err != nil {
		return fmt.Errorf("Unable stat %v: %v", fpath, err)
	}

	if flabel, err = Matchpathcon(fpath, fs.Mode()); flabel == "" {
		return fmt.Errorf("Unable to get context for %v: %v", fpath, err)
	}

	if rc, err := Setfilecon(fpath, flabel); rc != 0 {
		return fmt.Errorf("Unable to set selinux context for %v: %v", fpath, err)
	}

	return nil
}

func Test() {
	var plabel, flabel string
	if !SelinuxEnabled() {
		return
	}

	plabel, flabel = GetLxcContexts()
	fmt.Println(plabel)
	fmt.Println(flabel)
	freeContext(plabel)
	plabel, flabel = GetLxcContexts()
	fmt.Println(plabel)
	fmt.Println(flabel)
	freeContext(plabel)
	if SelinuxEnabled() {
		fmt.Println("Enabled")
	} else {
		fmt.Println("Disabled")
	}
	fmt.Println("getenforce ", SelinuxGetEnforce())
	fmt.Println("getenforcemode ", SelinuxGetEnforceMode())
	flabel, _ = Matchpathcon("/home/dwalsh/.emacs", 0)
	fmt.Println(flabel)
	pid := os.Getpid()
	fmt.Printf("PID:%d MCS:%s\n", pid, IntToMcs(pid, 1023))
	fmt.Println(Getcon())
	fmt.Println(Getfilecon("/etc/passwd"))
	fmt.Println(Getpidcon(1))
	Setfscreatecon("unconfined_u:unconfined_r:unconfined_t:s0")
	fmt.Println(Getfscreatecon())
	Setfscreatecon("")
	fmt.Println(Getfscreatecon())
	fmt.Println(Getpidcon(1))
}
