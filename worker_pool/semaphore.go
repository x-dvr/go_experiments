package workerpool

type SemaphorePool struct {
	sem chan struct{}
}

// check interface compliance
var _ Pool = &SemaphorePool{}

func NewSemaphorePool(cap int) *SemaphorePool {
	return &SemaphorePool{
		sem: make(chan struct{}, cap),
	}
}

func (p *SemaphorePool) Go(work Task) {
	p.sem <- struct{}{}
	go func() {
		defer func() {
			<-p.sem
		}()
		work()
	}()
}

func (p *SemaphorePool) Release() {}
