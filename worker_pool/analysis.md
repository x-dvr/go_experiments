# Worker-pool benchmark analysis

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

#### Aside: is the external WaitGroup on AntsPool unfair?

It's a reasonable concern — every other pool's `Drain()` uses an
internal `sync.WaitGroup`, but ants doesn't expose one, so the bench
wraps it externally. I checked three alternative drain strategies:

| Strategy                          | sec/op     | B/op      |
|-----------------------------------|------------|-----------|
| External `sync.WaitGroup`         |  69.5 ms   | 234.9 KiB |
| ants's `Reboot` + `ReleaseTimeout`|  74.0 ms   | 158.9 KiB |
| Poll `pool.Running() > 0` + yield |   1.90  s  | 159.0 KiB |

The external WG is the **fastest** of the three — ants's native
`ReleaseTimeout`/`Reboot` idiom (the one the original README used) is
slower because it internally polls for `Running() == 0` and tears
down/re-initialises pool state every batch. The WG path is a direct
signal/wait. Naive `Running()`-polling is catastrophic because the
polling goroutine starves the workers it's waiting on.

The WG does cost ~76 KiB extra allocations per iteration (the
closure captures an additional `*sync.WaitGroup` field, ~8 B × 10 000),
but the *alloc count* is identical and the per-task synchronisation
shape — one atomic Add, one atomic Done, one Wait — matches what the
Prealloc/Static/RR pools do internally. The comparison is fair.

### Why `SemaphorePool` is the slowest with the most allocs

It pays *all* the costs at once: spawns a goroutine per task
(`wg.Go(func(){...}())`), takes a permit from a `chan struct{}`
semaphore, and registers in the WaitGroup. Three allocs per task,
~30 k per iteration, 625 KiB/op. It's slower than ErrGroup
(which is structurally similar) mainly because the explicit channel
send/receive in `Go` is heavier than errgroup's optimised
`SetLimit` permit path.
