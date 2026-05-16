package sync

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
)

// PoolChanProducer recycles fixed-size byte slices via sync.Pool.
//
// Buffers are stored as *[]byte, not []byte. sync.Pool.Put/Get take/return
// `any`, which is a (type, data) pair where `data` is one pointer-sized
// word. A slice header is 24 bytes (ptr+len+cap), so storing a []byte
// forces the runtime to heap-allocate a copy of the slice header to back
// the `any` — one alloc per Put, defeating the pool. *[]byte fits in the
// interface data word directly, no boxing. (See staticcheck SA6002.)
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
