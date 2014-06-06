package tempfifo

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var rand uint32
var randmu sync.Mutex

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	randmu.Lock()
	r := rand
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	rand = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

func MkTempFifo(prefix, suffix string) (string, error) {
	var (
		err  error
		path string
	)
	nconflict := 0
	for i := 0; i < 10000; i++ {
		path = filepath.Join("/tmp", prefix+nextSuffix()+".sock")
		if err = syscall.Mknod(path, syscall.S_IFIFO|0666, 0); err != nil {
			if os.IsExist(err) {
				if nconflict++; nconflict > 10 {
					rand = reseed()
				}
				continue
			}
			return "", err
		}
		break
	}
	return path, nil
}

func RmTempFifo(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	} else {
		return nil
	}
}
