package sync

import (
	"sync"
)

type Buffer struct {
	buf [S64k]byte
	c   *sync.Cond
}

func NewBuffer() *Buffer {
	return &Buffer{
		c: sync.NewCond(&sync.Mutex{}),
	}
}

func (b *Buffer) LoadFrom(f func([]byte) error) {
	b.c.L.Lock()
	err := f(b.buf[:])
	b.c.L.Unlock()
	if err != nil {
		b.c.Signal()
		return
	}
	b.c.Signal()
}

func (b *Buffer) StoreTo(f func([]byte)) {
	b.c.L.Lock()
	b.c.Wait()
	f(b.buf[:])
	b.c.L.Unlock()
}
