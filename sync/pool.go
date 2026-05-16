package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
)

// PoolChanProducer recycles fixed-size byte slices via sync.Pool. Buffers
// are stored as *[]byte so the pool stores stable pointer-sized values
// (sync.Pool optimises for pointer-typed entries).
type PoolChanProducer struct {
	P sync.Pool
}

func NewPoolProd() *PoolChanProducer {
	return &PoolChanProducer{
		P: sync.Pool{
			New: func() any {
				b := make([]byte, S64k)
				return &b
			},
		},
	}
}

func (p *PoolChanProducer) Start(cnt int) <-chan *[]byte {
	ch := make(chan *[]byte, 1)
	go func() {
		defer close(ch)
		f, err := os.Open("/dev/urandom")
		if err != nil {
			return
		}
		defer f.Close()
		for range cnt {
			buf := p.P.Get().(*[]byte)
			if _, err := io.ReadFull(f, *buf); err != nil {
				p.P.Put(buf)
				return
			}
			ch <- buf
		}
	}()
	return ch
}

func ConsumePooled(ch <-chan *[]byte, p *sync.Pool) int {
	resCh := make(chan int)
	go func() {
		res := 0
		for buf := range ch {
			sum := sha256.Sum256(*buf)
			p.Put(buf)
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()
	return <-resCh
}
