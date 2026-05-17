package workerpool_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// Bench 3 (from analysis.md "Suggested next experiments"):
// Compare against runtime.LockOSThread()-pinned workers to see if
// the Go scheduler is the bottleneck on the shared-channel variants.
// If pinning helps Static (shared channel) more than RR (per-worker
// channel), scheduler-induced wakeup latency is meaningful; if not,
// hchan contention is the real cost.

type pinnedTask struct {
	Arg  int
	Iter int
}

type pinnedStaticPool struct {
	tasks   chan pinnedTask
	workers sync.WaitGroup
	pending sync.WaitGroup
}

func newPinnedStaticPool(task func(int, int), cap int) *pinnedStaticPool {
	p := &pinnedStaticPool{tasks: make(chan pinnedTask, cap)}
	for range cap {
		p.workers.Go(func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			for arg := range p.tasks {
				task(arg.Arg, arg.Iter)
				p.pending.Done()
			}
		})
	}
	return p
}

func (p *pinnedStaticPool) Go(arg, iter int) {
	p.pending.Add(1)
	p.tasks <- pinnedTask{Arg: arg, Iter: iter}
}

func (p *pinnedStaticPool) Drain() { p.pending.Wait() }

func (p *pinnedStaticPool) Release() {
	close(p.tasks)
	p.workers.Wait()
}

type pinnedRRPool struct {
	args    []chan pinnedTask
	cap     int64
	idx     atomic.Int64
	workers sync.WaitGroup
	pending sync.WaitGroup
}

func newPinnedRRPool(task func(int, int), cap int64) *pinnedRRPool {
	p := &pinnedRRPool{
		args: make([]chan pinnedTask, cap),
		cap:  cap,
	}
	for i := range cap {
		p.args[i] = make(chan pinnedTask, cap)
		p.workers.Go(func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			for arg := range p.args[i] {
				task(arg.Arg, arg.Iter)
				p.pending.Done()
			}
		})
	}
	return p
}

func (p *pinnedRRPool) Go(arg, iter int) {
	p.pending.Add(1)
	idx := p.idx.Add(1)
	p.args[idx%p.cap] <- pinnedTask{Arg: arg, Iter: iter}
}

func (p *pinnedRRPool) Drain() { p.pending.Wait() }

func (p *pinnedRRPool) Release() {
	for i := range p.cap {
		close(p.args[i])
	}
	p.workers.Wait()
}

func BenchmarkPinnedStaticPool(b *testing.B) {
	pool := newPinnedStaticPool(func(ct, iter int) {
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

func BenchmarkPinnedRoundRobinPool(b *testing.B) {
	pool := newPinnedRRPool(func(ct, iter int) {
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
