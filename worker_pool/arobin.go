package workerpool

import (
	"sync"
	"sync/atomic"
)

type aligned struct {
	Ch    chan tsk
	align [56]byte
}

type ARRPool struct {
	args []aligned
	cap  int64
	idx  atomic.Int64
	wg   sync.WaitGroup
}

func NewARRPool(task func(int, int), cap int64) *ARRPool {
	pool := ARRPool{
		args: make([]aligned, cap),
		cap:  cap,
	}

	for i := range cap {
		pool.args[i].Ch = make(chan tsk, 1)
		pool.wg.Go(func() {
			for arg := range pool.args[i].Ch {
				task(arg.Arg, arg.Iter)
			}
		})
	}

	return &pool
}

func (p *ARRPool) Go(arg, iter int) {
	idx := p.idx.Add(1)
	p.args[idx%p.cap].Ch <- tsk{Arg: arg, Iter: iter}
}

func (p *ARRPool) Release() {
	for i := range p.cap {
		close(p.args[i].Ch)
	}
}

func (p *ARRPool) Wait() {
	p.wg.Wait()
}
