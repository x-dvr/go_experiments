package fanin_test

import (
	"runtime"
	"testing"

	"github.com/x-dvr/go_experiments/fanin"
)

func TestCanonical(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount, makeBuffered[int](1))

	out := fanin.MergeCanonical(r...)

	send(w, msgCount, func(i int) int { return i })

	sum := 0
	for i := 0; i < msgCount; i++ {
		v := <-out
		sum += v
	}

	if sum != 40 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 40)
	}

	closeAll(w)

	cnt := 0
	for range out {
		cnt += 1
	}
	if cnt > 0 {
		t.Errorf("received more messages than sent: %d messages", cnt)
	}
}

func TestReflect(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount, makeBuffered[int](1))

	out := fanin.MergeReflect(r...)

	send(w, msgCount, func(i int) int { return i })

	sum := 0
	for i := 0; i < msgCount; i++ {
		v := <-out
		sum += v
	}

	if sum != 40 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 40)
	}

	closeAll(w)

	cnt := 0
	for range out {
		cnt += 1
	}
	if cnt > 0 {
		t.Errorf("received more messages than sent: %d messages", cnt)
	}
}

func TestLoop(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount, makeBuffered[int](1))

	out := fanin.MergeLoop(r...)

	send(w, msgCount, func(i int) int { return i })

	sum := 0
	for i := 0; i < msgCount; i++ {
		v := <-out
		sum += v
	}

	if sum != 40 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 40)
	}

	closeAll(w)

	cnt := 0
	for range out {
		cnt += 1
	}
	if cnt > 0 {
		t.Errorf("received more messages than sent: %d messages", cnt)
	}
}

func TestBatch4(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount, makeBuffered[int](1))

	out := fanin.MergeBatch4(r...)

	send(w, msgCount, func(i int) int { return i })

	sum := 0
	for i := 0; i < msgCount; i++ {
		v := <-out
		sum += v
	}

	if sum != 40 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 40)
	}

	closeAll(w)

	cnt := 0
	for range out {
		cnt += 1
	}
	if cnt > 0 {
		t.Errorf("received more messages than sent: %d messages", cnt)
	}
}

func TestBatch2(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount, makeBuffered[int](1))

	out := fanin.MergeBatch2(r...)

	send(w, msgCount, func(i int) int { return i })

	sum := 0
	for i := 0; i < msgCount; i++ {
		v := <-out
		sum += v
	}

	if sum != 40 {
		t.Errorf("got wrong value: %d, expected: %d", sum, 40)
	}

	closeAll(w)

	cnt := 0
	for range out {
		cnt += 1
	}
	if cnt > 0 {
		t.Errorf("received more messages than sent: %d messages", cnt)
	}
}

var sink int
var rSink Result

func BenchmarkWorkerPoolCanonical(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := 200
	w, r := setupSources(srcCount, makeBuffered[Result](100))

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) Result { return Result{Val: i} })
		rSink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolReflect(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := 200
	w, r := setupSources(srcCount, makeBuffered[Result](100))

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) Result { return Result{Val: i} })
		rSink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolLoop(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := 200
	w, r := setupSources(srcCount, makeBuffered[Result](100))

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) Result { return Result{Val: i} })
		rSink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolBatch4(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := 200
	w, r := setupSources(srcCount, makeBuffered[Result](100))

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) Result { return Result{Val: i} })
		rSink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolBatch2(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := 200
	w, r := setupSources(srcCount, makeBuffered[Result](100))

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) Result { return Result{Val: i} })
		rSink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsCanonical(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount, makeBuffered[int](5))

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsReflect(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount, makeBuffered[int](5))

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsLoop(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount, makeBuffered[int](5))

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsBatch4(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount, makeBuffered[int](5))

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsBatch2(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount, makeBuffered[int](5))

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountCanonical(b *testing.B) {
	srcCount := 1000
	msgCount := 100000
	w, r := setupSources(srcCount, makeBuffered[int](10))

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountReflect(b *testing.B) {
	srcCount := 1000
	msgCount := 100000
	w, r := setupSources(srcCount, makeBuffered[int](10))

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountBatch4(b *testing.B) {
	srcCount := 1000
	msgCount := 100000
	w, r := setupSources(srcCount, makeBuffered[int](10))

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountBatch2(b *testing.B) {
	srcCount := 1000
	msgCount := 100000
	w, r := setupSources(srcCount, makeBuffered[int](10))

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountLoop(b *testing.B) {
	srcCount := 1000
	msgCount := 100000
	w, r := setupSources(srcCount, makeBuffered[int](10))

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount, func(i int) int { return i })
		sink = read(out, msgCount)
	}

	closeAll(w)
}

type Result struct {
	Err error
	Val int
}

type MakeSource[T any] func() chan T

func makeBuffered[T any](size int) MakeSource[T] {
	return func() chan T {
		return make(chan T, size)
	}
}

func setupSources[T any](srcCount int, makeSource MakeSource[T]) ([]chan<- T, []<-chan T) {
	writable := make([]chan<- T, srcCount)
	readable := make([]<-chan T, srcCount)
	for idx := range srcCount {
		ch := makeSource()
		writable[idx] = ch
		readable[idx] = ch
	}

	return writable, readable
}

func closeAll[T any](ch []chan<- T) {
	for _, c := range ch {
		close(c)
	}
}

func send[T any](ch []chan<- T, count int, makeValue func(int) T) {
	goroutineCount := len(ch)
	// amount of messages sent by each goroutine
	batchSize := count / (goroutineCount - 1)
	lastBatch := count - batchSize*(goroutineCount-1)
	for i := range goroutineCount {
		size := batchSize
		if i == goroutineCount-1 {
			size = lastBatch
		}
		go func() {
			for k := range size {
				ch[i] <- makeValue(k)
			}
		}()
	}
}

func read[T any](ch <-chan T, count int) T {
	var sink T
	for i := 0; i < count; i++ {
		sink = <-ch
	}
	return sink
}
