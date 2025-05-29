package fanin_test

import (
	"runtime"
	"testing"

	"github.com/x-dvr/go_experiments/fanin"
)

func TestCanonical(t *testing.T) {
	srcCount := 5
	msgCount := 20
	w, r := setupSources(srcCount)

	out := fanin.MergeCanonical(r...)

	send(w, msgCount)

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
	w, r := setupSources(srcCount)

	out := fanin.MergeReflect(r...)

	send(w, msgCount)

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
	w, r := setupSources(srcCount)

	out := fanin.MergeLoop(r...)

	send(w, msgCount)

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
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch4(r...)

	send(w, msgCount)

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
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch2(r...)

	send(w, msgCount)

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

func BenchmarkWorkerPoolCanonical(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := runtime.NumCPU() * 10
	w, r := setupSources(srcCount)

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolReflect(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := runtime.NumCPU() * 10
	w, r := setupSources(srcCount)

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolLoop(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := runtime.NumCPU() * 10
	w, r := setupSources(srcCount)

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolBatch4(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := runtime.NumCPU() * 10
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkWorkerPoolBatch2(b *testing.B) {
	srcCount := runtime.NumCPU()
	msgCount := runtime.NumCPU() * 10
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsCanonical(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount)

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsReflect(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount)

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsLoop(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount)

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsBatch4(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkMetricsBatch2(b *testing.B) {
	srcCount := 100
	msgCount := 1000
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountCanonical(b *testing.B) {
	srcCount := 1000
	msgCount := 10000
	w, r := setupSources(srcCount)

	out := fanin.MergeCanonical(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountReflect(b *testing.B) {
	srcCount := 1000
	msgCount := 10000
	w, r := setupSources(srcCount)

	out := fanin.MergeReflect(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountBatch4(b *testing.B) {
	srcCount := 1000
	msgCount := 10000
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch4(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountBatch2(b *testing.B) {
	srcCount := 1000
	msgCount := 10000
	w, r := setupSources(srcCount)

	out := fanin.MergeBatch2(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func BenchmarkHugeSourceCountLoop(b *testing.B) {
	srcCount := 1000
	msgCount := 10000
	w, r := setupSources(srcCount)

	out := fanin.MergeLoop(r...)

	for b.Loop() {
		send(w, msgCount)
		sink = read(out, msgCount)
	}

	closeAll(w)
}

func setupSources(srcCount int) ([]chan<- int, []<-chan int) {
	writable := make([]chan<- int, srcCount)
	readable := make([]<-chan int, srcCount)
	for idx := range srcCount {
		ch := make(chan int, 5)
		writable[idx] = ch
		readable[idx] = ch
	}

	return writable, readable
}

func closeAll(ch []chan<- int) {
	for _, c := range ch {
		close(c)
	}
}

func send(ch []chan<- int, count int) {
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
				ch[i] <- k
			}
		}()
	}
}

func read(ch <-chan int, count int) int {
	var sink int
	for i := 0; i < count; i++ {
		sink = <-ch
	}
	return sink
}
