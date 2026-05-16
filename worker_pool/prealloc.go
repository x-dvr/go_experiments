package workerpool

import "sync"

type PreallocPool struct {
	tasks   chan Task
	workers sync.WaitGroup
	pending sync.WaitGroup
}

var _ Pool = (*PreallocPool)(nil)

func NewPreallocPool(cap int) *PreallocPool {
	pool := PreallocPool{
		tasks: make(chan Task, cap),
	}

	for range cap {
		pool.workers.Go(func() {
			for job := range pool.tasks {
				job()
				pool.pending.Done()
			}
		})
	}

	return &pool
}

func (p *PreallocPool) Go(w Task) {
	p.pending.Add(1)
	p.tasks <- w
}

func (p *PreallocPool) Drain() {
	p.pending.Wait()
}

func (p *PreallocPool) Release() {
	close(p.tasks)
	p.workers.Wait()
}
