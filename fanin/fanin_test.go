package fanin_test

import (
	"testing"

	"github.com/x-dvr/go_experiments/fanin"
)

func TestMergeGoChanInt(t *testing.T) {
	ch := prepare(100, 100)
	out := fanin.MergeGoChan(ch...)
	sum := 0
	for v := range out {
		sum += v
	}
	if sum != 495000 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 495000)
	}
}

func TestMergeLoopIterInt(t *testing.T) {
	ch := prepare(100, 100)
	out := fanin.MergeLoopIter(ch...)
	sum := 0
	for v := range out {
		sum += v
	}
	if sum != 495000 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 495000)
	}
}

func TestMergeReflectChanInt(t *testing.T) {
	ch := prepare(100, 100)
	out := fanin.MergeReflectChan(ch...)
	sum := 0
	for v := range out {
		sum += v
	}
	if sum != 495000 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 495000)
	}
}

func TestMergeReflectIterInt(t *testing.T) {
	ch := prepare(100, 100)
	out := fanin.MergeReflectIter(ch...)
	sum := 0
	for v := range out {
		sum += v
	}
	if sum != 495000 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 495000)
	}
}

var sink int

func BenchmarkMergeGoChanInt(b *testing.B) {
	ch := prepare(1_000, 2_000)
	out := fanin.MergeGoChan(ch...)
	for b.Loop() {
		sum := 0
		for v := range out {
			sum += v
		}
		sink = sum
	}
}

func BenchmarkMergeLoopIterInt(b *testing.B) {
	ch := prepare(1_000, 2_000)
	out := fanin.MergeLoopIter(ch...)
	for b.Loop() {
		sum := 0
		for v := range out {
			sum += v
		}
		sink = sum
	}
}

func BenchmarkMergeReflectChanInt(b *testing.B) {
	ch := prepare(100, 950)
	out := fanin.MergeReflectChan(ch...)
	for b.Loop() {
		sum := 0
		for v := range out {
			sum += v
		}
		sink = sum
	}
}

func BenchmarkMergeReflectIterInt(b *testing.B) {
	ch := prepare(100, 950)
	out := fanin.MergeReflectIter(ch...)
	for b.Loop() {
		sum := 0
		for v := range out {
			sum += v
		}
		sink = sum
	}
}

func prepare(countCh int, countMsg int) []<-chan int {
	chans := make([]chan int, countCh)
	for idx := range chans {
		chans[idx] = make(chan int)
	}
	tmp := make([]<-chan int, countCh)
	for idx := range chans {
		tmp[idx] = chans[idx]
	}

	for i := 0; i < countCh; i++ {
		go func() {
			for j := 0; j < countMsg; j++ {
				chans[i] <- j
			}
			close(chans[i])
		}()
	}

	return tmp
}
