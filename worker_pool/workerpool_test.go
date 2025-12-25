package workerpool_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"

	workerpool "github.com/x-dvr/go_experiments/worker_pool"
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

func BenchmarkRoundAlignedRobinPool(b *testing.B) {
	for b.Loop() {
		pool := workerpool.NewARRPool(func(ct, iter int) {
			sink[iter] = workHard(ct)
		}, int64(PoolCap))
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Release()
		pool.Wait()
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
		pool.Wait()
	}
}

func BenchmarkAntsPool(b *testing.B) {
	pool, _ := ants.NewPool(PoolCap, ants.WithPreAlloc(true))

	for b.Loop() {
		pool.Reboot()
		for i := range RunTimes {
			pool.Submit(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		pool.ReleaseTimeout(1 * time.Hour)
	}
}

func BenchmarkSemaphorePool(b *testing.B) {
	pool := workerpool.NewSemaphorePool(PoolCap)

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		pool.Wait()
	}

	pool.Release()
}

func BenchmarkPreallocPool(b *testing.B) {
	for b.Loop() {
		pool := workerpool.NewPreallocPool(PoolCap)
		for i := range RunTimes {
			pool.Go(func() {
				sink[i] = workHard(CalcTo)
			})
		}
		pool.Release()
		pool.Wait()
	}
}

func BenchmarkStaticPool(b *testing.B) {
	for b.Loop() {
		pool := workerpool.NewStaticPool(func(ct, iter int) {
			sink[iter] = workHard(ct)
		}, PoolCap)
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Release()
		pool.Wait()
	}
}

func BenchmarkRoundRobinPool(b *testing.B) {
	for b.Loop() {
		pool := workerpool.NewRRPool(func(ct, iter int) {
			sink[iter] = workHard(ct)
		}, int64(PoolCap))
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Release()
		pool.Wait()
	}
}
