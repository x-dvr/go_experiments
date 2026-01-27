package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
)

type CondProducer struct {
	buf1   [S64k]byte
	c1     *sync.Cond
	ready1 bool
	buf2   [S64k]byte
	c2     *sync.Cond
	ready2 bool
	done   bool
}

func NewCondProducer() *CondProducer {
	return &CondProducer{
		c1: sync.NewCond(&sync.Mutex{}),
		c2: sync.NewCond(&sync.Mutex{}),
	}
}

func (p *CondProducer) Start(cnt int) int {
	resCh := make(chan int, 2)
	p.done = false
	p.ready1 = false
	p.ready2 = false

	f, err := os.Open("/dev/urandom")
	if err != nil {
		return -1
	}
	defer f.Close()

	setDone := func() {
		p.c1.L.Lock()
		p.done = true
		p.ready1 = true
		p.c1.L.Unlock()
		p.c1.Signal()

		p.c2.L.Lock()
		p.done = true
		p.ready2 = true
		p.c2.L.Unlock()
		p.c2.Signal()
	}

	// producer
	go func() {
		for range cnt / 2 {
			_, err := io.ReadFull(f, p.buf1[:])
			p.c1.L.Lock()
			p.ready1 = true
			p.c1.L.Unlock()
			p.c1.Signal()
			if err != nil {
				setDone()
				return
			}

			_, err = io.ReadFull(f, p.buf2[:])
			p.c2.L.Lock()
			p.ready2 = true
			p.c2.L.Unlock()
			p.c2.Signal()
			if err != nil {
				setDone()
				return
			}
		}
		setDone()
	}()

	// consumer 1
	go func() {
		res := 0
		for {
			p.c1.L.Lock()
			for !p.ready1 {
				p.c1.Wait()
			}
			if p.done {
				p.c1.L.Unlock()
				break
			}
			sum := sha256.Sum256(p.buf1[:])
			p.ready1 = false
			p.c1.L.Unlock()
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
			for !p.ready2 {
				p.c2.Wait()
			}
			if p.done {
				p.c2.L.Unlock()
				break
			}
			sum := sha256.Sum256(p.buf2[:])
			p.ready2 = false
			p.c2.L.Unlock()
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
