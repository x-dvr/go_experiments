package workerpool

import "sync"

type SemaphorePool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

// check interface compliance
var _ Pool = (*SemaphorePool)(nil)

func NewSemaphorePool(cap int) *SemaphorePool {
	return &SemaphorePool{
		sem: make(chan struct{}, cap),
	}
}

func (p *SemaphorePool) Go(work Task) {
	p.sem <- struct{}{}
	p.wg.Go(func() {
		defer func() {
			<-p.sem
		}()
		work()
	})
}

func (p *SemaphorePool) Release() {
	close(p.sem)
}

func (p *SemaphorePool) Wait() {
	p.wg.Wait()
}
