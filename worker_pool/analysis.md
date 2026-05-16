# Worker-pool benchmark analysis

## Bug analysis

### 1. `arobin.go` cache-line padding is pointless

```go
type aligned struct {
    Ch    chan tsk    // 8 bytes — pointer to runtime.hchan
    align [56]byte
}
```

`Ch` is just a *pointer* to the heap-allocated `runtime.hchan`. The slice of
`aligned` stores pointers; the contended state (head/tail/buf, lock) lives
in the hchans those pointers reach. Padding between pointers does nothing
— the pointers are read-only after init. To actually eliminate false
sharing between channel buffers you'd need to pad inside each hchan or
roll your own MPMC ring; `make(chan T, N)` allocates exactly
`sizeof(hchan)+N*sizeof(T)` with no caller control over alignment.

The 4–9ms ARR-beats-RR gap in the results is **not** the padding helping —
it's run-to-run noise (benchstat reports `±∞` because `-count=3` is below
the 6 samples needed for a confidence interval).

### 2. Unfair pool lifecycle across benchmarks

`BenchmarkAntsPool` / `ErrGroup` / `SemaphorePool` construct the pool
**outside** `b.Loop()` (amortising pool setup over all `b.N` iterations);
`BenchmarkPreallocPool` / `StaticPool` / `RoundRobinPool` / `ARRPool`
construct it **inside** (paying ~32 goroutine spawns + 32 hchan
allocations per iter, repeatedly).

The 1.7 KiB / 6 KiB / 8 KiB allocations the no-alloc pools show are
entirely this per-iter pool construction — not the workload.

### 3. README results are stale

The README claims `NoPool: 43 ms, 100k allocs/op`. Current run:
`70 ms, 20k allocs/op`. Two changes account for this:

- Go 1.25's `sync.WaitGroup.Go(fn)` replaces the old
  `wg.Add(1); go func(){ defer wg.Done(); ... }()` pattern; the new form
  allocates ~5× less per task.
- `b.Loop()` (Go 1.24+) replaced what was probably a
  `for i := 0; i < b.N; i++` form; allocation accounting can differ
  slightly.

The README needs regenerating — the *rankings* differ too (NoPool was
1.6× faster than AntsPool then; now they're within 6%).

### 4. `StaticPool` / `RRPool` / `ARRPool` channel buffer is 1, not `PoolCap`

`make(chan tsk, 1)` forces the submitter goroutine to block every time a
worker is mid-task. Not a correctness bug, but it serialises submission.
With `cap = PoolCap` the producer can fan out a burst before blocking;
the current code measures the cost of synchronous handoff, not the cost
of the pool.

### 5. `SemaphorePool.Release()` is `close(p.sem)` after a `Wait()`

OK as written (no `Go` calls happen after the close), but if anyone
copies this pattern with concurrent producers, `p.sem <- struct{}{}` on
a closed channel panics. The Release/Wait/Go state machine isn't
documented and isn't enforced.

### Non-bugs that look like bugs

- `for i := range RunTimes { wg.Go(func() { sink[i] = ... }) }` — loop
  variable capture is fine: Go 1.22+ makes each iteration's `i`
  distinct, and the project is on Go 1.25.
- `pool.Release(); pool.Wait()` — `Release` closes the input channel,
  signalling workers to exit their `for range`; `Wait` blocks for them.
  Correct ordering.

---

## Results — explained

Numbers from `go test -bench=. -benchtime=10s -count=3` aggregated with
`benchstat`:

| Bench              | sec/op  | B/op    | allocs/op | per-task allocs |
|--------------------|--------:|--------:|----------:|:----------------|
| `NoPool`           | 70.3 ms | 395 KiB | 20,010    | 2 — closure + WG state |
| `AntsPool`         | 74.9 ms | 159 KiB | 10,060    | 1 — closure (workers reused) |
| `ErrGroup`         | 76.0 ms | 391 KiB | 20,000    | 2 — closure + group state |
| `RoundAlignedRobin`| 81.0 ms |   8 KiB |      99   | 0 (tsk struct, not closure) |
| `SemaphorePool`    | 78.0 ms | 625 KiB | 30,000    | 3 — closure + sem-send + WG |
| `PreallocPool`     | 77.9 ms | 158 KiB | 10,070    | 1 — closure (workers reused) |
| `StaticPool`       | 78.3 ms | 1.7 KiB |      66   | 0 (tsk struct) |
| `RoundRobinPool`   | 85.4 ms |   6 KiB |      98   | 0 (tsk struct) |

The 66/98/99 allocations for the `tsk`-struct pools are NOT per task —
they're per `b.N` iteration: `PoolCap=32` hchan allocations + worker
goroutine stack frames + a small fixed cost. Per *task* they're zero,
because `tsk{arg,iter}` is sent by value over the channel and the
closure `func(int,int)` is captured once at `NewStaticPool` time.

**Why everything clusters between 70–85 ms:**
`workHard(1e5)` does ~100k integer adds — roughly 200 µs of real work
per task. 10,000 tasks × 200 µs / 16 logical CPUs ≈ 125 ms of pure
compute, but the i7-10870H has 8 physical cores × HT, and Fibonacci
doesn't stress execution units, so wall-clock work is ~70 ms. Pool
overhead contributes at most ~15 ms — the entire spread between
fastest (NoPool) and slowest (RoundRobinPool).

**Why `NoPool` is fastest:**
Spawning 10,000 goroutines costs ~2–3 ms total; the scheduler then runs
them NumCPU at a time. There's no synchronization edge per task.
Pool-based variants pay a channel send/recv (mutex inside hchan) per
task — that's the ~5–10 ms extra they show.

**Why round-robin is *slowest*:**
Static dispatch (`tasks[i % cap] <- task`) ignores worker load. If one
worker is slow on its current task, its inbox queues while idle workers
do nothing. Static-pool's shared channel naturally load-balances because
*any* idle worker can grab the next task. The ARR/RR ~5–15 ms penalty
over Static is exactly this load-imbalance cost. Cache-line padding
doesn't recover any of it (see Bug 1).

**Why `AntsPool` matches the in-loop pools rather than beating them:**
Ants reuses workers (saves goroutine-spawn cost amortised over the loop),
but its internal task queue is mutex-protected — the contention is the
same shape as a shared channel. The pre-allocated workers help with
allocs (10k vs 20k) but not wall-clock.

**Why `SemaphorePool` has the most allocs but isn't proportionally slower:**
It pays a sem-send + a fresh goroutine + a WG entry per task (3 allocs),
but goroutine creation is cheap and the work itself dominates. The
allocator pressure shows in B/op (625 KiB — highest) but doesn't
translate into much extra runtime.

---

## Suggested next steps if you want to fix it

1. Move all pool constructions outside of `b.Loop()` for an
   apples-to-apples comparison. Ants's `Reboot` semantic isn't
   equivalent to a fresh `NewPool`.
2. Run with `-count=6` minimum so benchstat reports confidence intervals;
   the `± ∞` is silently telling you the current 3-sample run is not
   statistically meaningful.
3. Drop the `aligned` struct or pad the actual contended state. As-is it
   just bloats the slice.
4. Bump the channel buffer in Static/RR/ARR from 1 to `PoolCap` (or
   higher) and rerun — you'll likely see the gap to NoPool narrow.
5. Regenerate the README results — the existing table is from a code
   state that no longer exists.
