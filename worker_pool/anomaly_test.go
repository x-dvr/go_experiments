package workerpool_test

import (
	"sync"
	"testing"

	workerpool "github.com/0xde86/go_experiments/worker_pool"
)

// Investigates why CapSweepRoundRobin/cap=32 (~72 ms) is ~33% slower
// than top-level BenchmarkRoundRobinPool (~55 ms) with the same pool
// config. Two suspected variables on RR and Static:
//   1. method-value indirection (`submit := p.Go; submit(...)`)
//   2. b.Run subtest scaffolding vs top-level benchmark
// NoPool serves as a control — it has no pool method to indirect
// through, so any A→C delta on NoPool must come from b.Run alone.

// ---------- NoPool: A (top-level), C (subtest) ----------

func BenchmarkAnomalyNoPool_A_Top(b *testing.B) {
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

func BenchmarkAnomalyNoPool_C_Sub(b *testing.B) {
	var wg sync.WaitGroup
	b.Run("only", func(b *testing.B) {
		for b.Loop() {
			for i := range RunTimes {
				wg.Go(func() {
					sink[i] = workHard(CalcTo)
				})
			}
			wg.Wait()
		}
	})
}

// ---------- StaticPool: A, B, C, D ----------

func BenchmarkAnomalyStatic_A_TopDirect(b *testing.B) {
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

func BenchmarkAnomalyStatic_B_TopIndirect(b *testing.B) {
	pool := workerpool.NewStaticPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, PoolCap)
	defer pool.Release()
	submit := pool.Go
	drain := pool.Drain
	for b.Loop() {
		for i := range RunTimes {
			submit(CalcTo, i)
		}
		drain()
	}
}

func BenchmarkAnomalyStatic_C_SubDirect(b *testing.B) {
	pool := workerpool.NewStaticPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, PoolCap)
	defer pool.Release()
	b.Run("only", func(b *testing.B) {
		for b.Loop() {
			for i := range RunTimes {
				pool.Go(CalcTo, i)
			}
			pool.Drain()
		}
	})
}

func BenchmarkAnomalyStatic_D_SubIndirect(b *testing.B) {
	pool := workerpool.NewStaticPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, PoolCap)
	defer pool.Release()
	submit := pool.Go
	drain := pool.Drain
	b.Run("only", func(b *testing.B) {
		for b.Loop() {
			for i := range RunTimes {
				submit(CalcTo, i)
			}
			drain()
		}
	})
}

// ---------- RoundRobinPool: A, B, C, D ----------

func BenchmarkAnomalyRR_A_TopDirect(b *testing.B) {
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

func BenchmarkAnomalyRR_B_TopIndirect(b *testing.B) {
	pool := workerpool.NewRRPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, int64(PoolCap))
	defer pool.Release()
	submit := pool.Go
	drain := pool.Drain
	for b.Loop() {
		for i := range RunTimes {
			submit(CalcTo, i)
		}
		drain()
	}
}

func BenchmarkAnomalyRR_C_SubDirect(b *testing.B) {
	pool := workerpool.NewRRPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, int64(PoolCap))
	defer pool.Release()
	b.Run("only", func(b *testing.B) {
		for b.Loop() {
			for i := range RunTimes {
				pool.Go(CalcTo, i)
			}
			pool.Drain()
		}
	})
}

func BenchmarkAnomalyRR_D_SubIndirect(b *testing.B) {
	pool := workerpool.NewRRPool(func(ct, iter int) {
		sink[iter] = workHard(ct)
	}, int64(PoolCap))
	defer pool.Release()
	submit := pool.Go
	drain := pool.Drain
	b.Run("only", func(b *testing.B) {
		for b.Loop() {
			for i := range RunTimes {
				submit(CalcTo, i)
			}
			drain()
		}
	})
}
