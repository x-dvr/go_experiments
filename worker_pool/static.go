package workerpool

import "sync"

type StaticPool struct {
	tasks chan tsk
	wg    sync.WaitGroup
}

func NewStaticPool(task func(int, int), cap int) *StaticPool {
	pool := StaticPool{
		tasks: make(chan tsk, 1),
	}

	for range cap {
		pool.wg.Go(func() {
			for arg := range pool.tasks {
				task(arg.Arg, arg.Iter)
			}
		})
	}

	return &pool
}

func (p *StaticPool) Go(arg int, iter int) {
	p.tasks <- tsk{Arg: arg, Iter: iter}
}

func (p *StaticPool) Release() {
	close(p.tasks)
}

func (p *StaticPool) Wait() {
	p.wg.Wait()
}
