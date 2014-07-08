package apnsd

import (
	"sync"
)

type Identifier struct {
	m sync.Mutex
	i uint32
}

//TODO: generate uniq identifier in 4 bytes or reset
func (i *Identifier) NextIdentifier() uint32 {
	defer i.m.Unlock()
	i.m.Lock()
	i.i++
	return i.i
}
