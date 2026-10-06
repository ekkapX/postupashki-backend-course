package rwmutex

import (
	"primitives/internal/futex"
	"sync/atomic"
)

const (
	writer     = 1 << 31
	readerMask = ^uint32(writer)
)

type RWMutex struct {
	state uint32
}

func (rw *RWMutex) RLock() {
	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&writer != 0 {
			futex.Wait(&rw.state, cur)
			continue
		}
		if atomic.CompareAndSwapUint32(&rw.state, cur, cur+1) {
			return
		}
	}
}

func (rw *RWMutex) RUnlock() {
	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&readerMask == 0 {
			panic("RUnlock of unlocked RWMutex")
		}
		newVal := cur - 1
		if atomic.CompareAndSwapUint32(&rw.state, cur, newVal) {
			if newVal&readerMask == 0 && newVal&writer != 0 {
				futex.WakeAll(&rw.state)
			}
			return
		}
	}
}

func (rw *RWMutex) Lock() {
	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&writer != 0 {
			futex.Wait(&rw.state, cur)
			continue
		}
		if atomic.CompareAndSwapUint32(&rw.state, cur, cur|writer) {
			break
		}
	}

	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&readerMask == 0 {
			return
		}
		futex.Wait(&rw.state, cur)
	}
}

func (rw *RWMutex) Unlock() {
	if !atomic.CompareAndSwapUint32(&rw.state, writer, 0) {
		panic("Unlock of unlocked RWMutex")
	}
	futex.WakeAll(&rw.state)
}
