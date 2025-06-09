package workerpool

type RRPool struct {
	args []chan int
	cap  int
	idx  int
}

func NewRRPool(task func(int), cap, argsCap int) *RRPool {
	pool := RRPool{
		args: make([]chan int, cap),
		cap:  cap,
	}

	for i := range cap {
		pool.args[i] = make(chan int, argsCap)
		go func(args <-chan int) {
			for arg := range args {
				task(arg)
			}
		}(pool.args[i])
	}

	return &pool
}

func (p *RRPool) Go(arg int) {
	// this it not safe to call from different goroutines
	// if you want to call from multiple goroutines need to use atomic for idx
	p.idx = (p.idx + 1) % p.cap
	p.args[p.idx] <- arg
}

func (p *RRPool) Release() {
	for i := range p.cap {
		close(p.args[i])
	}
}
