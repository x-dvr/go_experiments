package workerpool

import (
	"sync"
	"sync/atomic"
)

type RRPool struct {
	args []chan tsk
	cap  int64
	idx  atomic.Int64
	wg   sync.WaitGroup
}

func NewRRPool(task func(int, int), cap int64) *RRPool {
	pool := RRPool{
		args: make([]chan tsk, cap),
		cap:  cap,
	}

	for i := range cap {
		pool.args[i] = make(chan tsk, 1)
		pool.wg.Go(func() {
			for arg := range pool.args[i] {
				task(arg.Arg, arg.Iter)
			}
		})
	}

	return &pool
}

func (p *RRPool) Go(arg, iter int) {
	idx := p.idx.Add(1)
	p.args[idx%p.cap] <- tsk{Arg: arg, Iter: iter}
}

func (p *RRPool) Release() {
	for i := range p.cap {
		close(p.args[i])
	}
}

func (p *RRPool) Wait() {
	p.wg.Wait()
}
