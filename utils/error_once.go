package utils

import (
	"sync"
)

type ErrorOnce struct {
	once sync.Once
	err  error
}

func (i *ErrorOnce) Error(f func() error) error {
	i.once.Do(func() {
		i.err = f()
	})
	return i.err
}
