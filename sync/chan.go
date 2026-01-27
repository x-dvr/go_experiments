package sync

import (
	"crypto/sha256"
	"io"
	"os"
)

type ChanProducer struct{}

func (p ChanProducer) Start(cnt int) <-chan []byte {
	ch := make(chan []byte, 1)
	go func() {
		defer close(ch)
		f, err := os.Open("/dev/urandom")
		if err != nil {
			return
		}
		defer f.Close()
		for range cnt {
			buf := make([]byte, S64k)
			if _, err := io.ReadFull(f, buf); err != nil {
				return
			}
			ch <- buf
		}
	}()
	return ch
}

func Consume(ch <-chan []byte) int {
	resCh := make(chan int)
	go func() {
		res := 0
		for buf := range ch {
			sum := sha256.Sum256(buf)
			for i := range len(sum) {
				res += int(sum[i])
			}
		}
		resCh <- res
	}()
	return <-resCh
}
