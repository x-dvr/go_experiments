package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// RingProducer is a single-producer / single-consumer ring buffer.
// head and tail are isolated on separate cache lines to avoid producer/
// consumer false sharing.
//
// NOTE: portable Go has no way to align a heap-allocated struct to a
// cache-line boundary — Go's allocator guarantees 8-byte alignment, not 64.
// The inter-field padding below ensures head and tail don't share a line
// with each other, but the struct's leading edge may share a line with
// whatever heap data precedes it. For an SPSC use case that's fine — only
// head/tail mutual isolation matters.
type RingProducer struct {
	head     atomic.Uint64
	_pad0    [56]byte
	tail     atomic.Uint64
	_pad1    [56]byte
	done     atomic.Bool
	_pad2    [63]byte
	slots    []RingSlot
	size     uint64
	notEmpty *sync.Cond
	notFull  *sync.Cond
}

type RingSlot struct {
	buf [S64k]byte
}

func NewRingProducer(size int) *RingProducer {
	r := &RingProducer{
		slots:    make([]RingSlot, size),
		size:     uint64(size),
		notEmpty: sync.NewCond(&sync.Mutex{}),
		notFull:  sync.NewCond(&sync.Mutex{}),
	}
	return r
}

func (r *RingProducer) Start(cnt int) int {
	r.head.Store(0)
	r.tail.Store(0)
	r.done.Store(false)

	f, err := os.Open("/dev/urandom")
	if err != nil {
		return -1
	}
	defer f.Close()

	// producer
	go func() {
		for range cnt {
			head := r.head.Load()

			r.notFull.L.Lock()
			for head-r.tail.Load() >= r.size {
				r.notFull.Wait()
			}
			r.notFull.L.Unlock()

			_, err := io.ReadFull(f, r.slots[head%r.size].buf[:])
			if err != nil {
				r.done.Store(true)
				r.notEmpty.Signal()
				return
			}

			r.head.Store(head + 1)
			r.notEmpty.Signal()
		}
		r.done.Store(true)
		r.notEmpty.Signal()
	}()

	// consumer
	res := 0
	for {
		tail := r.tail.Load()

		r.notEmpty.L.Lock()
		for tail == r.head.Load() && !r.done.Load() {
			r.notEmpty.Wait()
		}
		r.notEmpty.L.Unlock()

		if tail == r.head.Load() && r.done.Load() {
			return res
		}

		sum := sha256.Sum256(r.slots[tail%r.size].buf[:])
		r.tail.Store(tail + 1)
		r.notFull.Signal()

		for i := range len(sum) {
			res += int(sum[i])
		}
	}
}
