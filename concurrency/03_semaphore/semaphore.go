package semaphore

import (
	"math"
	"primitives/internal/futex"
	"sync/atomic"
)

type Semaphore struct {
	permits uint32
	waiters uint32
}

func New(n int) *Semaphore {
	if n < 0 || n > math.MaxInt32 {
		panic("semaphore: invalid initial permits")
	}
	return &Semaphore{permits: uint32(n)}
}

func (s *Semaphore) tryDecrement() bool {
	for {
		cur := atomic.LoadUint32(&s.permits)
		if cur == 0 {
			return false
		}
		if atomic.CompareAndSwapUint32(&s.permits, cur, cur-1) {
			return true
		}
	}
}

func (s *Semaphore) Acquire() {
	for {
		if s.tryDecrement() {
			return
		}
		atomic.AddUint32(&s.waiters, 1)
		futex.Wait(&s.permits, 0)
		atomic.AddUint32(&s.waiters, ^uint32(0))
	}
}

func (s *Semaphore) TryAcquire() bool {
	return s.tryDecrement()
}

func (s *Semaphore) Release() {
	atomic.AddUint32(&s.permits, 1)
	if atomic.LoadUint32(&s.waiters) > 0 {
		futex.Wake(&s.permits)
	}
}

func (s *Semaphore) Available() int {
	return int(atomic.LoadUint32(&s.permits))
}
