package waitgroup

import (
	"math"
	"primitives/internal/futex"
	"sync/atomic"
)

type WaitGroup struct {
	count uint32
}

func (wg *WaitGroup) Add(delta int) {
	for {
		old := atomic.LoadUint32(&wg.count)
		n := int64(old) + int64(delta)
		if n < 0 {
			panic("waitgroup: negative counter")
		}
		if n > math.MaxInt32 {
			panic("waitgroup: counter overflow")
		}
		if atomic.CompareAndSwapUint32(&wg.count, old, uint32(n)) {
			if n == 0 {
				futex.WakeAll(&wg.count)
			}
			return
		}
	}
}

func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

func (wg *WaitGroup) Wait() {
	for {
		cur := atomic.LoadUint32(&wg.count)
		if cur == 0 {
			return
		}
		futex.Wait(&wg.count, cur)
	}
}
