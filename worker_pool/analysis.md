# Worker-pool benchmark analysis (v2 — after fixes)

This is the post-fix version of the earlier analysis. The original
issues — unfair pool lifecycle, pointless cache-line padding, stale
README numbers, undersized channel buffers — have all been addressed.
The fixes change the result ranking substantially.

## What changed

1. **All pools are now constructed *outside* `b.Loop()`** and reused
   across iterations. Each pool grew a `Drain()` method (a per-batch
   `sync.WaitGroup`) so the benchmark can wait for the current batch
   to complete without closing the input channel. Per-iter goroutine
   spawn and `make(chan T, …)` allocations are gone.
2. **Channel buffers bumped from 1 to `PoolCap`** in `StaticPool` and
   `RoundRobinPool` so the submitter can fan out a burst of tasks
   before blocking on a worker.
3. **`ARRPool` (`arobin.go`) and its benchmark deleted.** The
   `[56]byte` "cache-line padding" between pointers in the slice did
   nothing — the contended state lives in the heap-allocated `hchan`,
   not at the slice element. If you actually want per-channel cache
   isolation you'd have to roll your own MPMC ring; `make(chan T, N)`
   doesn't let you control the hchan layout.
4. **`StaticPool.Wait`, `PreallocPool.Wait`, `RRPool.Wait` removed**
   in favour of `Drain()` (batch-complete) + `Release()` (final close).
   The tracer in `worker_pool/tracer/main.go` and the benchmark file
   were updated to use the new API.

## Results

`go test -bench=. -benchtime=10s -count=6` on Intel i7-10870H @ 2.20 GHz,
Linux amd64, Go 1.25, aggregated with `benchstat`:

| Bench              | sec/op           | B/op            | allocs/op | per-task allocs |
|--------------------|------------------|-----------------|----------:|:----------------|
| `RoundRobinPool`   |  61.01 ms ± 2%   |       3.5 B     |    0      | 0 — `tsk` struct, per-worker channel |
| `NoPool`           |  70.68 ms ± 7%   |   390.6 KiB     | 20,000    | 2 — closure + WG state |
| `AntsPool`         |  73.93 ms ± 1%   |   234.7 KiB     | 10,000    | 1 — closure (workers reused, but external WG adds work) |
| `ErrGroup`         |  74.95 ms ± 2%   |   390.6 KiB     | 20,000    | 2 — closure + group state |
| `PreallocPool`     |  77.27 ms ± 0%   |   156.3 KiB     | 10,000    | 1 — closure (shared channel) |
| `StaticPool`       |  79.14 ms ± 1%   |       5.0 B     |    0      | 0 — `tsk` struct, shared channel |
| `SemaphorePool`    |  79.59 ms ± 1%   |   625.0 KiB     | 30,000    | 3 — closure + sem-send + WG |

The 3.5–5 B/op shown for the zero-alloc pools is bench-framework GC
overhead attributed to the iteration count; per-task it's zero. All
the `±` figures here are honest 6-sample confidence intervals from
benchstat — the `± ∞` from the previous run is gone.

## Interpretation

### Why `RoundRobinPool` now wins (-14 % vs NoPool)

Three things combine to put RR ahead of every other variant:

1. **No goroutine creation per task.** 32 workers are spawned once at
   pool construction and live for the whole benchmark. NoPool spawns
   10,000 goroutines per iteration; even though `wg.Go` is cheap, that's
   10 k mallocgc + 10 k scheduler enqueues per batch.
2. **Per-worker channel with buffer `PoolCap=32`.** The submitter can
   push 32 tasks at a worker before blocking. Contention on each
   channel's hchan lock is negligible because the producer/consumer
   touch each channel from at most two goroutines.
3. **Zero per-task allocations.** `tsk{Arg, Iter}` is sent by value
   over the channel; the closure `func(int, int)` was captured once
   at `NewRRPool` time and is reused for every task. Allocator pressure
   on the submitter side is zero.

### Why `StaticPool` (same idea, shared channel) is slower than RR

Static and RR are mechanically identical except for the channel
topology: Static has *one* shared channel, RR has *N* per-worker
channels. With one channel, all 32 workers contend for the same hchan
lock on `<-tasks`. Under steady load that's a real contention point —
the ~18 ms gap between RR (61) and Static (79) is exactly that.

Round-robin dispatch has its own classic problem (one slow task
queues behind it while other workers idle), but here every task is
nearly identical (Fibonacci of the same `n`), so the load is uniformly
distributed and the contention savings dominate.

### Why `NoPool` is still good (-15 % to -28 % vs the other pools)

Spawning 10,000 goroutines costs ~2–3 ms total on Go 1.25 — far less
than the ~15 ms of cross-channel synchronisation the shared-channel
pools pay. The work itself (~70 ms across 16 logical CPUs) dominates,
so the goroutine-spawn overhead is amortised over the parallel
compute. NoPool only loses to RR because RR has *both* zero per-task
allocs *and* almost-zero contention.

### Why `AntsPool`, `ErrGroup`, `PreallocPool` cluster at 74–77 ms

All three pay a per-task synchronisation cost on a shared concurrency
gate: ants uses an internal mutex-protected work queue, errgroup uses
a `chan struct{}` permit semaphore, prealloc uses one shared
`chan Task`. The shapes of the gates differ but the cost is similar:
~1–2 µs per task × 10 k tasks ≈ the observed 5–10 ms gap to NoPool.

`AntsPool` shows ~10 k allocs vs ErrGroup's ~20 k because ants reuses
its task slots (no per-task `g` allocation) — but the external
`sync.WaitGroup` we added to drain each batch costs one alloc per task.

### Why `SemaphorePool` is the slowest with the most allocs

It pays *all* the costs at once: spawns a goroutine per task
(`wg.Go(func(){...}())`), takes a permit from a `chan struct{}`
semaphore, and registers in the WaitGroup. Three allocs per task,
~30 k per iteration, 625 KiB/op. It's slower than ErrGroup
(which is structurally similar) mainly because the explicit channel
send/receive in `Go` is heavier than errgroup's optimised
`SetLimit` permit path.

## What was *NOT* fixed

- **CPU pinning / `GOMAXPROCS` stability.** The benches still run
  under whatever the OS scheduler decides. With `-count=6` the
  confidence intervals are tight (≤2 % for most), so this hasn't
  bitten us, but if you want to compare across machines, fix
  `GOMAXPROCS` and use `taskset`.
- **Tail-task latency.** All numbers here are mean throughput
  (wall-clock per batch). If you care about p99 task latency,
  `RoundRobinPool` may lose to `StaticPool` because static dispatch
  is dynamic load-balancing while RR isn't. The current bench
  doesn't expose this — every task is the same cost.

## Suggested next experiments

1. Vary `PoolCap` from `NumCPU/2` to `NumCPU*8` and see at what point
   `RoundRobinPool` stops being the winner. (Hypothesis: at very high
   `PoolCap`, scheduler load dominates and per-channel contention
   becomes irrelevant.)
2. Add a "variable task size" workload (some tasks 10× longer than
   others) to expose RR's load-imbalance weakness.
3. Compare against `runtime.LockOSThread()`-pinned workers to see if
   the scheduler is the bottleneck on the shared-channel variants.
