package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// CondProducer is a producer + two-consumer ping-pong using sync.Cond.
// The producer alternates writes between buf1 and buf2; consumer1 hashes
// buf1, consumer2 hashes buf2. The producer must wait for the previous
// consumption of a buffer (ready=false) before overwriting it — otherwise
// the read and write race.
type CondProducer struct {
	buf1   [S64k]byte
	c1     *sync.Cond
	ready1 bool
	buf2   [S64k]byte
	c2     *sync.Cond
	ready2 bool
	done   atomic.Bool
}

func NewCondProducer() *CondProducer {
	return &CondProducer{
		c1: sync.NewCond(&sync.Mutex{}),
		c2: sync.NewCond(&sync.Mutex{}),
	}
}

func (p *CondProducer) Start(cnt int) int {
	resCh := make(chan int, 2)
	p.done.Store(false)
	p.ready1 = false
	p.ready2 = false

	f, err := os.Open("/dev/urandom")
	if err != nil {
		return -1
	}
	defer f.Close()

	wakeAll := func() {
		p.done.Store(true)
		// atomic.Bool Store→Load provides happens-before across both
		// goroutines; no extra cond-lock fence needed before Signal.
		p.c1.Signal()
		p.c2.Signal()
	}

	// producer
	go func() {
		for range cnt / 2 {
			// Wait for consumer to release buf1 before overwriting it.
			p.c1.L.Lock()
			for p.ready1 {
				p.c1.Wait()
			}
			p.c1.L.Unlock()

			_, err := io.ReadFull(f, p.buf1[:])
			if err != nil {
				wakeAll()
				return
			}
			p.c1.L.Lock()
			p.ready1 = true
			p.c1.L.Unlock()
			p.c1.Signal()

			p.c2.L.Lock()
			for p.ready2 {
				p.c2.Wait()
			}
			p.c2.L.Unlock()

			_, err = io.ReadFull(f, p.buf2[:])
			if err != nil {
				wakeAll()
				return
			}
			p.c2.L.Lock()
			p.ready2 = true
			p.c2.L.Unlock()
			p.c2.Signal()
		}
		wakeAll()
	}()

	// consumer 1
	go func() {
		res := 0
		for {
			p.c1.L.Lock()
			for !p.ready1 && !p.done.Load() {
				p.c1.Wait()
			}
			if !p.ready1 && p.done.Load() {
				p.c1.L.Unlock()
				break
			}
			sum := sha256.Sum256(p.buf1[:])
			p.ready1 = false
			p.c1.L.Unlock()
			p.c1.Signal()
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()

	// consumer 2
	go func() {
		res := 0
		for {
			p.c2.L.Lock()
			for !p.ready2 && !p.done.Load() {
				p.c2.Wait()
			}
			if !p.ready2 && p.done.Load() {
				p.c2.L.Unlock()
				break
			}
			sum := sha256.Sum256(p.buf2[:])
			p.ready2 = false
			p.c2.L.Unlock()
			p.c2.Signal()
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()

	r1 := <-resCh
	r2 := <-resCh
	return r1 + r2
}
