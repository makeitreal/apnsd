package apnsd

import (
	"sync"
)

type Identifier struct {
	m sync.Mutex
	i uint32
}

func (i *Identifier) NextIdentifier() uint32 {
	defer i.m.Unlock()
	i.m.Lock()
	i.i++
	return i.i
}
