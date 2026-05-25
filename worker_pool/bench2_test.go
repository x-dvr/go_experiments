package workerpool_test

import (
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"

	workerpool "github.com/0xde86/go_experiments/worker_pool"
)

// Bench 2 (from analysis.md "Suggested next experiments"):
// Variable-cost task workload — every 10th task is 10x heavier. This
// exposes the static dispatch weakness of RoundRobinPool versus the
// shared-channel pools that dynamically load-balance: an unlucky RR
// worker queues up multiple heavy tasks while the others go idle.

const (
	HeavyMultiplier = 10
	HeavyEvery      = 10
)

func workVariable(iter, calcTo int) int {
	if iter%HeavyEvery == 0 {
		return workHard(calcTo * HeavyMultiplier)
	}
	return workHard(calcTo)
}

func BenchmarkVariableNoPool(b *testing.B) {
	var wg sync.WaitGroup
	for b.Loop() {
		for i := range RunTimes {
			wg.Go(func() {
				sink[i] = workVariable(i, CalcTo)
			})
		}
		wg.Wait()
	}
}

func BenchmarkVariableErrGroup(b *testing.B) {
	var pool errgroup.Group
	pool.SetLimit(PoolCap)

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() error {
				sink[i] = workVariable(i, CalcTo)
				return nil
			})
		}
		_ = pool.Wait()
	}
}

func BenchmarkVariableAntsPool(b *testing.B) {
	pool, _ := ants.NewPool(PoolCap, ants.WithPreAlloc(true))
	defer pool.ReleaseTimeout(1 * time.Hour)

	var wg sync.WaitGroup
	for b.Loop() {
		wg.Add(RunTimes)
		for i := range RunTimes {
			_ = pool.Submit(func() {
				sink[i] = workVariable(i, CalcTo)
				wg.Done()
			})
		}
		wg.Wait()
	}
}

func BenchmarkVariablePreallocPool(b *testing.B) {
	pool := workerpool.NewPreallocPool(PoolCap)
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(func() {
				sink[i] = workVariable(i, CalcTo)
			})
		}
		pool.Drain()
	}
}

func BenchmarkVariableStaticPool(b *testing.B) {
	pool := workerpool.NewStaticPool(func(ct, iter int) {
		sink[iter] = workVariable(iter, ct)
	}, PoolCap)
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Drain()
	}
}

func BenchmarkVariableRoundRobinPool(b *testing.B) {
	pool := workerpool.NewRRPool(func(ct, iter int) {
		sink[iter] = workVariable(iter, ct)
	}, int64(PoolCap))
	defer pool.Release()

	for b.Loop() {
		for i := range RunTimes {
			pool.Go(CalcTo, i)
		}
		pool.Drain()
	}
}
