package rwmutex

import (
	"primitives/internal/futex"
	"sync/atomic"
)

const (
	writer     = 1 << 31
	waiters    = 1 << 30
	readerMask = waiters - 1
)

type RWMutex struct {
	state uint32
}

func (rw *RWMutex) sleep(cur uint32) {
	if cur&waiters == 0 {
		if !atomic.CompareAndSwapUint32(&rw.state, cur, cur|waiters) {
			return
		}
		cur |= waiters
	}
	futex.Wait(&rw.state, cur)
}

func (rw *RWMutex) RLock() {
	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&writer != 0 {
			rw.sleep(cur)
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
		wake := newVal&readerMask == 0 && newVal&writer != 0 && newVal&waiters != 0
		if wake {
			newVal &^= waiters
		}
		if atomic.CompareAndSwapUint32(&rw.state, cur, newVal) {
			if wake {
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
			rw.sleep(cur)
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
		rw.sleep(cur)
	}
}

func (rw *RWMutex) Unlock() {
	for {
		cur := atomic.LoadUint32(&rw.state)
		if cur&^waiters != writer {
			panic("Unlock of unlocked RWMutex")
		}
		if atomic.CompareAndSwapUint32(&rw.state, cur, 0) {
			if cur&waiters != 0 {
				futex.WakeAll(&rw.state)
			}
			return
		}
	}
}
