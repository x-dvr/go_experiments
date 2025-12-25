package workerpool

import "sync"

type PreallocPool struct {
	tasks chan Task
	wg    sync.WaitGroup
}

// check interface compliance
var _ Pool = (*PreallocPool)(nil)

func NewPreallocPool(cap int) *PreallocPool {
	pool := PreallocPool{
		tasks: make(chan Task, 1),
	}

	for range cap {
		pool.wg.Go(func() {
			for job := range pool.tasks {
				job()
			}
		})
	}

	return &pool
}

func (p *PreallocPool) Go(w Task) {
	p.tasks <- w
}

func (p *PreallocPool) Release() {
	close(p.tasks)
}

func (p *PreallocPool) Wait() {
	p.wg.Wait()
}
