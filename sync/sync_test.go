package sync_test

import (
	"testing"

	"github.com/x-dvr/go_experiments/sync"
)

const cnt = 1_000

func TestSize(t *testing.T) {
	var b [sync.S64k * 2]byte

	b1 := b[:sync.S64k]
	b2 := b[sync.S64k:]

	if len(b1) != len(b2) || len(b1) != sync.S64k {
		t.Errorf("Wrong buffer length: %d != %d", len(b1), len(b2))
	}
}

func TestPoolProducer(t *testing.T) {
	p := sync.NewPoolProd()
	ch := p.Start(10)
	for b := range ch {
		if len(*b) != sync.S64k {
			t.Errorf("Wrong buffer read: %d", len(*b))
		}
		p.P.Put(b)
	}
}

func BenchmarkSeq(b *testing.B) {
	p := sync.SeqProducer{}

	for b.Loop() {
		_ = p.Start(cnt)
	}
}

func BenchmarkChan(b *testing.B) {
	p := sync.ChanProducer{}

	for b.Loop() {
		ch := p.Start(cnt)
		_ = sync.Consume(ch)
	}
}

func BenchmarkPoolChan(b *testing.B) {
	p := sync.NewPoolProd()

	for b.Loop() {
		ch := p.Start(cnt)
		_ = sync.ConsumePooled(ch, &p.P)
	}
}

func BenchmarkCond(b *testing.B) {
	p := sync.NewCondProducer()

	for b.Loop() {
		_ = p.Start(cnt)
	}
}

func BenchmarkRing(b *testing.B) {
	p := sync.NewRingProducer(4)

	for b.Loop() {
		_ = p.Start(cnt)
	}
}
