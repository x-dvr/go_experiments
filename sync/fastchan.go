package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// Chan is a bounded single-producer / single-consumer channel with an API
// similar to Go's built-in channels: Send/Recv block, TrySend/TryRecv are
// non-blocking, Close marks the channel done.
//
// Layout: head + producer-private cachedTail share one cache line; tail +
// consumer-private cachedHead share another. The fast path only reads
// the peer's atomic when the cached snapshot says we're out of room — in
// steady state both sides read mostly from their own L1 line, which
// avoids the cache-line ping-pong a naive head/tail Load on every op
// would cause. The mutex is touched only when a goroutine actually needs
// to park or wake its peer.
//
// SPSC is the supported shape — concurrent Senders or concurrent Recvs
// will corrupt the ring. Use one goroutine on each side.
type Chan[T any] struct {
	// producer cacheline (64B)
	head        atomic.Uint64 // 8
	cachedTail  uint64        // 8  producer-owned snapshot of tail
	sendWaiting atomic.Bool   // 4
	_pad0       [44]byte      // 44

	// consumer cacheline (64B)
	tail        atomic.Uint64 // 8
	cachedHead  uint64        // 8  consumer-owned snapshot of head
	recvWaiting atomic.Bool   // 4
	_pad1       [44]byte      // 44

	closed atomic.Bool
	mask   uint64
	cap    uint64
	slots  []T

	mu       sync.Mutex
	notFull  sync.Cond
	notEmpty sync.Cond
}

// NewChan returns a Chan whose capacity is the smallest power of two
// >= the requested capacity (minimum 1). Power-of-two capacity lets the
// ring index with a mask instead of a modulo.
func NewChan[T any](capacity int) *Chan[T] {
	if capacity < 1 {
		capacity = 1
	}
	size := uint64(1)
	for size < uint64(capacity) {
		size <<= 1
	}
	c := &Chan[T]{
		mask:  size - 1,
		cap:   size,
		slots: make([]T, size),
	}
	c.notFull.L = &c.mu
	c.notEmpty.L = &c.mu
	return c
}

// Cap returns the channel's capacity (rounded up to the next power of 2).
func (c *Chan[T]) Cap() int { return int(c.cap) }

// Len returns the current number of buffered items. The result is a
// snapshot and may be stale by the time the caller acts on it.
func (c *Chan[T]) Len() int {
	return int(c.head.Load() - c.tail.Load())
}

// TrySend tries to enqueue v without blocking. It returns false if the
// channel is full or closed.
func (c *Chan[T]) TrySend(v T) bool {
	if c.closed.Load() {
		return false
	}
	head := c.head.Load()
	if head-c.cachedTail >= c.cap {
		c.cachedTail = c.tail.Load()
		if head-c.cachedTail >= c.cap {
			return false
		}
	}
	c.slots[head&c.mask] = v
	c.head.Store(head + 1)
	c.wakeRecv()
	return true
}

// Send enqueues v, blocking until there is space. Panics if the channel
// is closed (matching Go channel semantics).
func (c *Chan[T]) Send(v T) {
	if c.closed.Load() {
		panic("send on closed Chan")
	}
	head := c.head.Load()
	if head-c.cachedTail >= c.cap {
		c.cachedTail = c.tail.Load()
	}
	if head-c.cachedTail < c.cap {
		c.slots[head&c.mask] = v
		c.head.Store(head + 1)
		c.wakeRecv()
		return
	}
	// Slow path. Avoid the lost-wakeup race: set sendWaiting=true BEFORE
	// re-reading tail. The consumer's fast path is tail.Store then
	// sendWaiting.Load — if both we and the consumer fail to observe
	// each other's store, the SC total order forces a cycle, which is
	// impossible. Either we observe the new tail (and don't park) or
	// the consumer observes sendWaiting=true (and wakes us).
	c.mu.Lock()
	c.sendWaiting.Store(true)
	for {
		head = c.head.Load()
		c.cachedTail = c.tail.Load()
		if head-c.cachedTail < c.cap {
			break
		}
		if c.closed.Load() {
			c.sendWaiting.Store(false)
			c.mu.Unlock()
			panic("send on closed Chan")
		}
		c.notFull.Wait()
	}
	c.sendWaiting.Store(false)
	c.slots[head&c.mask] = v
	c.head.Store(head + 1)
	c.mu.Unlock()
	c.wakeRecv()
}

// TryRecv pops the next value without blocking. Returns the value and
// true on success, or the zero value and false if the channel is empty
// (regardless of whether it is closed).
func (c *Chan[T]) TryRecv() (T, bool) {
	var zero T
	tail := c.tail.Load()
	if tail == c.cachedHead {
		c.cachedHead = c.head.Load()
		if tail == c.cachedHead {
			return zero, false
		}
	}
	v := c.slots[tail&c.mask]
	c.slots[tail&c.mask] = zero // drop reference for GC
	c.tail.Store(tail + 1)
	c.wakeSend()
	return v, true
}

// Recv pops the next value, blocking until one is available. Returns
// false if the channel has been closed and fully drained.
func (c *Chan[T]) Recv() (T, bool) {
	var zero T
	tail := c.tail.Load()
	if tail == c.cachedHead {
		c.cachedHead = c.head.Load()
	}
	if tail < c.cachedHead {
		v := c.slots[tail&c.mask]
		c.slots[tail&c.mask] = zero
		c.tail.Store(tail + 1)
		c.wakeSend()
		return v, true
	}
	// Slow path. Mirror of Send: store recvWaiting=true BEFORE reading
	// head, so the SC total order rules out a lost wakeup against the
	// producer's lock-free fast path (head.Store followed by
	// recvWaiting.Load).
	c.mu.Lock()
	c.recvWaiting.Store(true)
	for {
		c.cachedHead = c.head.Load()
		if tail < c.cachedHead {
			break
		}
		if c.closed.Load() {
			c.recvWaiting.Store(false)
			c.mu.Unlock()
			return zero, false
		}
		c.notEmpty.Wait()
	}
	c.recvWaiting.Store(false)
	v := c.slots[tail&c.mask]
	c.slots[tail&c.mask] = zero
	c.tail.Store(tail + 1)
	c.mu.Unlock()
	c.wakeSend()
	return v, true
}

// Close marks the channel closed and wakes any parked goroutines. Safe
// to call concurrently with Recv/TryRecv; calling Send after Close
// panics. Idempotent.
func (c *Chan[T]) Close() {
	c.mu.Lock()
	c.closed.Store(true)
	c.notEmpty.Broadcast()
	c.notFull.Broadcast()
	c.mu.Unlock()
}

// wakeRecv signals notEmpty iff the consumer has actually parked. The
// recvWaiting flag is set under c.mu before Wait, so once the consumer
// has decided to sleep its Store is visible to this Load (Go atomics
// are sequentially consistent). If we see false the consumer either
// already saw the new head or is about to — either way it makes
// progress without our help.
func (c *Chan[T]) wakeRecv() {
	if !c.recvWaiting.Load() {
		return
	}
	c.mu.Lock()
	c.notEmpty.Signal()
	c.mu.Unlock()
}

// wakeSend mirrors wakeRecv for the producer side.
func (c *Chan[T]) wakeSend() {
	if !c.sendWaiting.Load() {
		return
	}
	c.mu.Lock()
	c.notFull.Signal()
	c.mu.Unlock()
}

// FastChanProducer is the pipeline variant used by BenchmarkFastChan —
// same shape as ChanProducer, but the hand-off uses Chan[*[]byte].
type FastChanProducer struct{}

func (FastChanProducer) Start(cnt int) int {
	ch := NewChan[*[]byte](1)
	resCh := make(chan int, 1)

	go func() {
		defer ch.Close()
		f, err := os.Open("/dev/urandom")
		if err != nil {
			return
		}
		defer f.Close()
		for range cnt {
			buf := make([]byte, S64k)
			if _, err := io.ReadFull(f, buf); err != nil {
				return
			}
			ch.Send(&buf)
		}
	}()

	go func() {
		res := 0
		for {
			buf, ok := ch.Recv()
			if !ok {
				break
			}
			sum := sha256.Sum256(*buf)
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()
	return <-resCh
}
