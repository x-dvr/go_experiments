package sync_test

import (
	"testing"

	"github.com/0xde86/go_experiments/sync"
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

func BenchmarkFastChan(b *testing.B) {
	p := sync.FastChanProducer{}

	for b.Loop() {
		_ = p.Start(cnt)
	}
}

// Pure hand-off throughput — strips the /dev/urandom + SHA bottleneck
// that dominates the variants above, so the channel itself is what's
// measured. Three flavours probe different regimes:
//
//   - Throughput*: bare ping-pong, cap=64. Both sides are so fast that
//     the buffer constantly hits empty/full and the park/unpark cost
//     dominates — sync.Cond is heavier than the runtime's chan parker,
//     so we end up ~tied with Go chan here.
//   - Unbounded*: capacity larger than the item count, so Send never
//     blocks. Isolates the lock-free fast path — FastChan wins ~2×.
//   - Work*: a few hundred ns of arithmetic per item between ops, so
//     the buffer rarely bottoms out. Closer to a real pipeline shape;
//     FastChan wins ~20%.
const throughputN = 1_000_000

func BenchmarkThroughputGoChan(b *testing.B) {
	for b.Loop() {
		ch := make(chan int, 64)
		done := make(chan struct{})
		go func() {
			for range ch {
			}
			close(done)
		}()
		for i := range throughputN {
			ch <- i
		}
		close(ch)
		<-done
	}
}

func BenchmarkThroughputFastChan(b *testing.B) {
	for b.Loop() {
		ch := sync.NewChan[int](64)
		done := make(chan struct{})
		go func() {
			for {
				if _, ok := ch.Recv(); !ok {
					break
				}
			}
			close(done)
		}()
		for i := range throughputN {
			ch.Send(i)
		}
		ch.Close()
		<-done
	}
}

func BenchmarkUnboundedGoChan(b *testing.B) {
	for b.Loop() {
		ch := make(chan int, throughputN+1)
		for i := range throughputN {
			ch <- i
		}
		close(ch)
		for range ch {
		}
	}
}

func BenchmarkUnboundedFastChan(b *testing.B) {
	for b.Loop() {
		ch := sync.NewChan[int](throughputN + 1)
		for i := range throughputN {
			ch.Send(i)
		}
		ch.Close()
		for {
			if _, ok := ch.Recv(); !ok {
				break
			}
		}
	}
}

//go:noinline
func tinyWork(x int) int {
	r := x
	for i := 1; i < 50; i++ {
		r = r*31 + i
	}
	return r
}

func BenchmarkWorkGoChan(b *testing.B) {
	for b.Loop() {
		ch := make(chan int, 64)
		done := make(chan int, 1)
		go func() {
			sum := 0
			for v := range ch {
				sum += tinyWork(v)
			}
			done <- sum
		}()
		for i := range throughputN {
			_ = tinyWork(i)
			ch <- i
		}
		close(ch)
		<-done
	}
}

func BenchmarkWorkFastChan(b *testing.B) {
	for b.Loop() {
		ch := sync.NewChan[int](64)
		done := make(chan int, 1)
		go func() {
			sum := 0
			for {
				v, ok := ch.Recv()
				if !ok {
					break
				}
				sum += tinyWork(v)
			}
			done <- sum
		}()
		for i := range throughputN {
			_ = tinyWork(i)
			ch.Send(i)
		}
		ch.Close()
		<-done
	}
}

func TestFastChanSendRecv(t *testing.T) {
	ch := sync.NewChan[int](4)
	for i := range 4 {
		if !ch.TrySend(i) {
			t.Fatalf("TrySend(%d) failed", i)
		}
	}
	if ch.TrySend(99) {
		t.Fatalf("TrySend on full channel should fail")
	}
	for i := range 4 {
		v, ok := ch.TryRecv()
		if !ok || v != i {
			t.Fatalf("TryRecv: got (%d,%v), want (%d,true)", v, ok, i)
		}
	}
	if _, ok := ch.TryRecv(); ok {
		t.Fatalf("TryRecv on empty channel should fail")
	}
}

func TestFastChanCloseDrain(t *testing.T) {
	ch := sync.NewChan[int](2)
	ch.Send(1)
	ch.Send(2)
	ch.Close()
	v, ok := ch.Recv()
	if !ok || v != 1 {
		t.Fatalf("got (%d,%v), want (1,true)", v, ok)
	}
	v, ok = ch.Recv()
	if !ok || v != 2 {
		t.Fatalf("got (%d,%v), want (2,true)", v, ok)
	}
	if _, ok := ch.Recv(); ok {
		t.Fatalf("Recv after drain should return false")
	}
}

func TestFastChanBlockingSendRecv(t *testing.T) {
	ch := sync.NewChan[int](2)
	const n = 1000
	done := make(chan int, 1)
	go func() {
		sum := 0
		for {
			v, ok := ch.Recv()
			if !ok {
				break
			}
			sum += v
		}
		done <- sum
	}()
	want := 0
	for i := 1; i <= n; i++ {
		ch.Send(i)
		want += i
	}
	ch.Close()
	if got := <-done; got != want {
		t.Fatalf("sum: got %d want %d", got, want)
	}
}
