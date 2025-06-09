package workerpool_test

import (
	"runtime"
	"sync"
	"testing"

	"github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"

	workerpool "github.com/x-dvr/go_experiments/worker_pool"
)

var (
	CalcTo   int = 1e4
	RunTimes int = 1e5
	PoolCap  int = runtime.NumCPU() - 1
)

// var sink int

func workHard(calcTo int) {
	var n2, n1 = 0, 1
	for i := 2; i <= calcTo; i++ {
		n2, n1 = n1, n1+n2
	}
	// sink = n1
}

type worker struct {
	wg *sync.WaitGroup
}

func (w worker) Work() {
	workHard(CalcTo)
	w.wg.Done()
}

func (w worker) WorkTo(to int) {
	workHard(to)
	w.wg.Done()
}

func (w worker) WorkErr() error {
	workHard(CalcTo)
	w.wg.Done()
	return nil
}

func BenchmarkNoPool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			go w.Work()
		}
		wg.Wait()
	}
}

func BenchmarkErrGroup(b *testing.B) {
	var wg sync.WaitGroup
	var pool errgroup.Group
	w := worker{wg: &wg}
	pool.SetLimit(PoolCap)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Go(w.WorkErr)
		}
		wg.Wait()
	}
}

func BenchmarkAntsPool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}
	pool, _ := ants.NewPool(PoolCap)

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Submit(w.Work)
		}
		wg.Wait()
	}

	pool.Release()
}

func BenchmarkSemaphorePool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}
	pool := workerpool.NewSemaphorePool(PoolCap)

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Go(w.Work)
		}
		wg.Wait()
	}

	pool.Release()
}

func BenchmarkPreallocPool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}
	pool := workerpool.NewPreallocPool(PoolCap, RunTimes)

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Go(w.Work)
		}
		wg.Wait()
	}

	pool.Release()
}

func BenchmarkStaticPool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}
	pool := workerpool.NewStaticPool(w.WorkTo, PoolCap, RunTimes)

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Go(CalcTo)
		}
		wg.Wait()
	}

	pool.Release()
}

func BenchmarkRoundRobinPool(b *testing.B) {
	var wg sync.WaitGroup
	w := worker{wg: &wg}
	pool := workerpool.NewRRPool(w.WorkTo, PoolCap, RunTimes)

	for b.Loop() {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			pool.Go(CalcTo)
		}
		wg.Wait()
	}

	pool.Release()
}
