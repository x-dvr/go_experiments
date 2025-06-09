package workerpool

type StaticPool struct {
	tasks chan int
}

func NewStaticPool(task func(int), cap, taskCap int) *StaticPool {
	pool := StaticPool{
		tasks: make(chan int, taskCap),
	}

	for range cap {
		go func(args <-chan int) {
			for arg := range args {
				task(arg)
			}
		}(pool.tasks)
	}

	return &pool
}

func (p *StaticPool) Go(arg int) {
	p.tasks <- arg
}

func (p *StaticPool) Release() {
	close(p.tasks)
}
