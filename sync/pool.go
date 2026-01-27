package sync

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"sync"
)

type PoolChanProducer struct {
	P sync.Pool
}

func NewPoolProd() *PoolChanProducer {
	return &PoolChanProducer{
		P: sync.Pool{
			New: func() any {
				buf := make([]byte, 0, S64k)
				return bytes.NewBuffer(buf)
			},
		},
	}
}

func (p *PoolChanProducer) Start(cnt int) <-chan *bytes.Buffer {
	ch := make(chan *bytes.Buffer, 1)
	go func() {
		defer close(ch)
		f, err := os.Open("/dev/urandom")
		if err != nil {
			return
		}
		defer f.Close()
		for range cnt {
			buf := p.P.Get().(*bytes.Buffer)
			if _, err := io.CopyN(buf, f, S64k); err != nil {
				return
			}
			ch <- buf
		}
	}()
	return ch
}

func ConsumePooled(ch <-chan *bytes.Buffer, p *sync.Pool) int {
	resCh := make(chan int)
	go func() {
		res := 0
		for buf := range ch {
			sum := sha256.Sum256(buf.Bytes())
			buf.Reset()
			p.Put(buf)
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()
	return <-resCh
}
