package workerpool

import "sync"

type StaticPool struct {
	tasks   chan tsk
	workers sync.WaitGroup
	pending sync.WaitGroup
}

func NewStaticPool(task func(int, int), cap int) *StaticPool {
	pool := StaticPool{
		tasks: make(chan tsk, cap),
	}

	for range cap {
		pool.workers.Go(func() {
			for arg := range pool.tasks {
				task(arg.Arg, arg.Iter)
				pool.pending.Done()
			}
		})
	}

	return &pool
}

func (p *StaticPool) Go(arg int, iter int) {
	p.pending.Add(1)
	p.tasks <- tsk{Arg: arg, Iter: iter}
}

func (p *StaticPool) Drain() {
	p.pending.Wait()
}

func (p *StaticPool) Release() {
	close(p.tasks)
	p.workers.Wait()
}
