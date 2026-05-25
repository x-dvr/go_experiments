package workerpool_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"

	workerpool "github.com/0xde86/go_experiments/worker_pool"
)

const (
	CalcTo   int = 1e5
	RunTimes int = 1e4
)

var (
	PoolCap int   = runtime.NumCPU() * 2
	sink    []int = make([]int, RunTimes)
)

func workHard(calcTo int) int {
	var n2, n1 = 0, 1
	for i := 2; i <= calcTo; i++ {
		n2, n1 = n1, n1+n2
	}
	return n1
}

// All pool constructions are hoisted outside b.Loop() so each iteration
// measures only the submit-and-drain cost, not goroutine spawn / hchan
// allocation. Drain() per iteration waits for all submitted tasks of the
// current batch to complete without closing the input channel.

func BenchmarkNoPool(b *testing.B) {
	var wg sync.WaitGroup
	for b.Loop() {
		for i := range RunTimes {
			wg.Go(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		wg.Wait()
	}
}

func BenchmarkErrGroup(b *testing.B) {
	var pool errgroup.Group
	pool.SetLimit(PoolCap)

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() error {
				sink[i] = workHard(CalcTo)
				return nil
			})
		}
		_ = pool.Wait()
	}
}

func BenchmarkAntsPool(b *testing.B) {
	pool, _ := ants.NewPool(PoolCap, ants.WithPreAlloc(true))
	defer pool.ReleaseTimeout(1 * time.Hour)

	var wg sync.WaitGroup
	for b.Loop() {
		wg.Add(RunTimes)
		for i := range RunTimes {
			_ = pool.Submit(func() {
				sink[i] = workHard(CalcTo)
				wg.Done()
			})
		}
		wg.Wait()
	}
}

func BenchmarkSemaphorePool(b *testing.B) {
	pool := workerpool.NewSemaphorePool(PoolCap)
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		pool.Wait()
	}
}

func BenchmarkPreallocPool(b *testing.B) {
	pool := workerpool.NewPreallocPool(PoolCap)
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		pool.Drain()
	}
}

func BenchmarkStaticPool(b *testing.B) {
	pool := workerpool.NewStaticPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, PoolCap)
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Drain()
	}
}

func BenchmarkRoundRobinPool(b *testing.B) {
	pool := workerpool.NewRRPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, int64(PoolCap))
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Drain()
	}
}
