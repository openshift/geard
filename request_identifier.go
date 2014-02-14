package geard

import (
	"container/list"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
)

type RequestIdentifier []byte

func (r RequestIdentifier) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(r), "=")
}

func (r RequestIdentifier) UnitNameFor() string {
	return fmt.Sprintf("job-%s.service", r.ToShortName())
}

func (r RequestIdentifier) UnitNameForBuild() string {
	return fmt.Sprintf("build-%s.service", r.ToShortName())
}

type RequestIdentifierMap struct {
	keys  map[string]interface{}
	order *list.List
	max   int
	lock  sync.RWMutex
}

func NewRequestIdentifierMap(size int) *RequestIdentifierMap {
	return &RequestIdentifierMap{
		keys:  make(map[string]interface{}, size),
		order: list.New(),
		max:   size,
	}
}

func (m RequestIdentifierMap) Get(id RequestIdentifier) interface{} {
	key := string(id)

	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.keys[key]
}

func (m RequestIdentifierMap) Put(id RequestIdentifier, v interface{}) (interface{}, bool) {
	key := string(id)

	m.lock.Lock()
	defer m.lock.Unlock()

	if existing, contains := m.keys[key]; contains {
		if v == nil {
			m.keys[key] = nil
		}
		return existing, true
	}

	if m.order.Len() > m.max {
		last := m.order.Back()
		m.order.Remove(last)
		id := last.Value.(string)
		delete(m.keys, id)
	}
	m.order.PushFront(key)
	m.keys[key] = v
	return nil, false
}
