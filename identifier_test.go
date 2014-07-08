package apnsd

import (
	"sync"
	"testing"
)

func TestIdentifier(t *testing.T) {

	var wg sync.WaitGroup
	identifier := &Identifier{}

	m := make(map[uint32]int)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				n := identifier.NextIdentifier()
				m[n]++
			}
		}()
	}

	wg.Wait()

	i := identifier.NextIdentifier()

	for i := 1; i <= 20; i++ {
		if m[uint32(i)] != 1 {
			t.Error("i:", i, "should be 1")
		}
	}

	if i != 21 {
		t.Error("i should be 21")
	}

	identifier.i = ^uint32(0)

	if identifier.NextIdentifier() != uint32(0) {
		t.Error("NextIdentifier should be 0")
	}

	if identifier.NextIdentifier() != uint32(1) {
		t.Error("NextIdentifier should be 1")
	}
}
