package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

type RingProducer struct {
	head     atomic.Uint64 // next write position (only producer writes)
	_pad0    [56]byte
	tail     atomic.Uint64 // next read position (only consumer writes)
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
