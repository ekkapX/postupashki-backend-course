package once

import (
	"primitives/internal/futex"
	"sync/atomic"
)

type Once struct {
	state uint32
}

const (
	notStarted = iota
	running
	done
)

func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.state) == done {
		return
	}

	if atomic.CompareAndSwapUint32(&o.state, notStarted, running) {
		defer func() {
			atomic.StoreUint32(&o.state, done)
			futex.WakeAll(&o.state)
		}()
		f()
		return
	}

	for {
		cur := atomic.LoadUint32(&o.state)
		if cur == done {
			return
		}
		futex.Wait(&o.state, cur)
	}
}

func (o *Once) Done() bool {
	return atomic.LoadUint32(&o.state) == done
}
