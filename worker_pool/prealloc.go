package workerpool

type PreallocPool struct {
	tasks chan Task
}

// check interface compliance
var _ Pool = &PreallocPool{}

func NewPreallocPool(cap, taskCap int) *PreallocPool {
	pool := PreallocPool{
		tasks: make(chan Task, taskCap),
	}

	for range cap {
		go func(jobs <-chan Task) {
			for job := range jobs {
				job()
			}
		}(pool.tasks)
	}

	return &pool
}

func (p *PreallocPool) Go(w Task) {
	p.tasks <- w
}

func (p *PreallocPool) Release() {
	close(p.tasks)
}
