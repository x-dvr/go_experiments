package workerpool_test

import (
	"runtime"
	"strconv"
	"testing"

	workerpool "github.com/x-dvr/go_experiments/worker_pool"
)

// Bench 1 (from analysis.md "Suggested next experiments"):
// Vary PoolCap from NumCPU/2 to NumCPU*8 and see at what point
// RoundRobinPool stops being the winner. Hypothesis: at very high
// PoolCap, scheduler load dominates and per-channel contention
// becomes irrelevant.

func benchCapSweep(b *testing.B, build func(cap int) (submit func(arg, iter int), drain, release func())) {
	ncpu := runtime.NumCPU()
	for _, cap := range []int{ncpu / 2, ncpu, ncpu * 2, ncpu * 4, ncpu * 8} {
		submit, drain, release := build(cap)
		b.Run("cap="+strconv.Itoa(cap), func(b *testing.B) {
			for b.Loop() {
				for i := range RunTimes {
					submit(CalcTo, i)
				}
				drain()
			}
		})
		release()
	}
}

func BenchmarkCapSweepRoundRobin(b *testing.B) {
	benchCapSweep(b, func(cap int) (func(int, int), func(), func()) {
		p := workerpool.NewRRPool(func(ct, iter int) {
			sink[iter] = workHard(ct)
		}, int64(cap))
		return p.Go, p.Drain, p.Release
	})
}

func BenchmarkCapSweepStatic(b *testing.B) {
	benchCapSweep(b, func(cap int) (func(int, int), func(), func()) {
		p := workerpool.NewStaticPool(func(ct, iter int) {
			sink[iter] = workHard(ct)
		}, cap)
		return p.Go, p.Drain, p.Release
	})
}
