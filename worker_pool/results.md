# Worker-pool benchmark results

All numbers below come from **isolated benchmark runs with a 60 s
cooldown** between each invocation. Each `go test -bench=<one>` runs
in its own process so the runtime, GC pacer, and CPU thermal state
don't carry across pools. See `README.md` for the exact commands.

Earlier suite-style runs (`go test -bench=.` with everything in one
process) gave wildly different rankings depending on what ran first —
StaticPool measured at 48 ms in isolation, 52 ms after `NoPool`, and
71 ms after `PreallocPool` in the same binary. None of those numbers
are wrong; they just measure different things. **The numbers below
are the ones to use for comparing pool implementations.**

## Environment

- Intel(R) Core(TM) i7-10870H @ 2.20 GHz, 16 logical CPUs
- Linux amd64, Go 1.25
- `RunTimes = 10 000` tasks of `workHard(1e5)` per iteration
- `PoolCap = runtime.NumCPU() * 2 = 32` for cases 0 / 2 / 3
- Per bench: `-benchmem -benchtime=5s -count=6`, aggregated with
  `benchstat`
- 60 s `sleep` before every `go test` invocation

---

## Case 0 — Baseline, uniform task cost

| Bench            | sec/op           | B/op         | allocs/op |
|------------------|------------------|--------------|----------:|
| `StaticPool`     |  48.59 ms ± 3%   |       ~0     |       0   |
| `AntsPool`       |  64.18 ms ± 3%   | 235.0 KiB    |  10 010   |
| `RoundRobinPool` |  64.20 ms ± 3%   |       ~0     |       0   |
| `PreallocPool`   |  64.42 ms ± 3%   | 156.3 KiB    |  10 000   |
| `NoPool`         |  65.99 ms ± 3%   | 390.8 KiB    |  20 000   |
| `SemaphorePool`  |  66.23 ms ± 4%   | 625.1 KiB    |  30 000   |
| `ErrGroup`       |  66.72 ms ± 4%   | 390.6 KiB    |  20 000   |

The ~0 B/op entries on `StaticPool` and `RoundRobinPool` are
bench-framework noise — per-task allocations are zero.

### Observations

1. **`StaticPool` dominates** by ~24 % over every other pool — it
   submits a `tsk{Arg, Iter}` value into one shared channel with no
   per-task closure allocation, and the runtime hands the next task
   to whichever worker is ready. Effectively a tiny work-stealing
   queue.

2. **Every other pool clusters at 64–67 ms**, regardless of how it's
   built: `RoundRobinPool` (per-worker channels, zero alloc),
   `AntsPool` (shared queue + 10 k allocs), `ErrGroup` (permit channel
   + 20 k allocs), `NoPool` (10 k goroutine spawns). The total
   wall-clock is dominated by `workHard` plus the scheduler /
   channel-sync overhead common to all of them; allocator pressure is
   not the deciding factor at this scale.

3. **`RoundRobinPool` is *not* the winner** under uniform load —
   contrary to what an earlier suite-style measurement showed. The
   theoretical advantage of RR (per-worker channels → no hchan
   contention) is outweighed by its static dispatch: the submitter
   waits for `args[idx%N]` specifically, while Static lets the
   runtime pick any ready worker.

---

## Case 1 — `PoolCap` sweep

Each cap is run within a single `b.Run` subtest tree, so subtests
within one bench function don't get individual cooldowns. Numbers
within each pool are internally comparable; absolute numbers vs
Case 0 should be treated with mild scepticism because of subtest
scaffolding (see notes below).

### `RoundRobinPool`

| cap          | sec/op           |
|--------------|------------------|
| `NumCPU/2`   |  93.56 ms ± 1%   |
| `NumCPU`     |  51.84 ms ± 3%   |
| `NumCPU*2`   |  51.73 ms ± 1%   |
| `NumCPU*4`   |  51.86 ms ± 0%   |
| `NumCPU*8`   |  52.53 ms ± 1%   |

### `StaticPool`

| cap          | sec/op           |
|--------------|------------------|
| `NumCPU/2`   |  71.55 ms ± 2%   |
| `NumCPU`     |  60.79 ms ± 1%   |
| `NumCPU*2`   |  51.98 ms ± 0%   |
| `NumCPU*4`   |  52.35 ms ± 0%   |
| `NumCPU*8`   |  52.64 ms ± 0%   |

### Observations

1. **At `cap = NumCPU/2` RR collapses** to 94 ms — every worker is
   busy and the submitter blocks waiting for the *specific* worker
   the round-robin index targets. Static (71 ms) loses less because
   its single shared channel still lets the submitter offload to
   whichever worker the runtime wakes next.

2. **At `cap = NumCPU` the two pools cross**: RR is now ~15 %
   faster than Static (52 vs 61 ms). Below saturation the
   "no-contention" property starts mattering.

3. **At `cap ≥ NumCPU*2` both pools converge to ~52 ms** and stay
   flat through `NumCPU*8 = 128`. Once there are more workers than
   logical CPUs, workers spend most of their time parked on `<-tasks`,
   so the hchan lock that hurt Static at low cap is essentially
   uncontended and the two pools are equivalent at the channel level.

4. **No scheduler-saturation degradation up to cap=128.** To actually
   find a breaking point where scheduler load dominates you'd need
   thousands of workers, not hundreds.

Note: cap-sweep RR / Static at cap = 32 (≈ 52 ms) is faster than
standalone `RoundRobinPool` (64 ms) in Case 0. This is the same code
path. The bench is one function with five back-to-back subtests, and
the first subtest (cap=8) burns 30 s of CPU before cap=32 runs; by
the time cap=32 measures, the runtime is in a different steady state
than a freshly-launched process. Same workload, different runtime
warm-up state — another illustration that absolute pool comparisons
need each pool in its own process.

---

## Case 2 — Variable task workload

Every 10th task uses `workHard(1e6)` (10× iterations) instead of
`workHard(1e5)`. Total work ≈ 1.9× uniform.

| Bench                       | sec/op           | B/op         | allocs/op |
|-----------------------------|------------------|--------------|----------:|
| `VariableRoundRobinPool`    |  78.78 ms ± 3%   |       ~0     |       0   |
| `VariableErrGroup`          |  79.60 ms ± 4%   | 390.7 KiB    |  20 000   |
| `VariablePreallocPool`      |  97.06 ms ± 3%   | 156.3 KiB    |  10 000   |
| `VariableAntsPool`          |  98.86 ms ± 3%   | 235.0 KiB    |  10 000   |
| `VariableNoPool`            |  99.41 ms ± 3%   | 390.8 KiB    |  20 000   |
| `VariableStaticPool`        | 101.50 ms ± 3%   |       ~0     |       0   |

### Observations

1. **The Case 0 ranking inverts completely.** `StaticPool` — the
   winner under uniform load — is now the *slowest* pool, and
   `RoundRobinPool` (mid-pack in Case 0) is the *fastest*. ErrGroup
   is within 1 % of RR.

2. **Why RR wins:** with `PoolCap = 32` and heavy-every-10 indexing,
   half of the per-worker channels (the odd-indexed ones, by the
   submitter's `idx.Add(1)` counter) end up with every heavy task.
   That sounds bad, but it means the 16 "heavy" workers and the 16
   "light" workers each finish their stream in roughly the time of
   the slowest worker — and the per-worker channel ops never contend
   with anything else. No lock contention, ever.

3. **Why Static loses:** with one shared channel feeding 32 workers,
   every recv contends on the same hchan lock. Under variable cost,
   workers are constantly desynchronised, so the recv-side lock is
   acquired and released continually rather than absorbing a burst
   into the buffer. The "dynamic load balancing at the channel" trick
   that helps Static under *uniform* load doesn't compensate for the
   increased lock traffic under *variable* load.

4. **ErrGroup is a strong second** for the same reason RR wins: each
   limit-permit-released goroutine receives one task and exits, so
   the permit channel is a one-shot hand-off rather than a contended
   queue.

5. **NoPool / AntsPool / PreallocPool cluster at 97–99 ms** —
   shared-queue dispatch with per-task closure allocation. Their
   wall-clock is dominated by the same hchan contention that hurts
   Static, plus closure-alloc cost.

---

## Case 3 — `LockOSThread()`-pinned workers

| Bench                       | sec/op           |
|-----------------------------|------------------|
| `PinnedStaticPool`          |  61.51 ms ± 2%   |
| `PinnedRoundRobinPool`      |  78.75 ms ± 1%   |

Side-by-side against Case 0:

| Pool             | Non-pinned       | Pinned           |  Δ                 |
|------------------|------------------|------------------|--------------------|
| `StaticPool`     |  48.59 ms ± 3%   |  61.51 ms ± 2%   | **+27 %** (worse)  |
| `RoundRobinPool` |  64.20 ms ± 3%   |  78.75 ms ± 1%   | **+23 %** (worse)  |

### Observations

1. **Pinning hurts both pools by ~25 %.** The Go scheduler is *not*
   the bottleneck on either shared-channel or per-worker-channel
   dispatch. Removing the scheduler's ability to migrate worker
   goroutines onto free Ms (e.g. when GC needs a thread, or when a
   worker is parked on a syscall) measurably hurts throughput. This
   is consistent with conventional Go guidance: `LockOSThread` is
   for cgo / syscall / thread-affine APIs, not throughput tuning.

2. **The Static-vs-RR gap is preserved.** Both pools degrade by
   similar proportions, so pinning doesn't differentially help the
   shared-channel variant. The fact that Static-with-pinning still
   beats RR-with-pinning by the same margin tells us the Static win
   in Case 0 isn't a scheduler artifact — it's the property of the
   shared queue itself.

---

## Cross-case summary

| Workload                       | Winner               | Loser                | Gap   |
|--------------------------------|----------------------|----------------------|-------|
| Uniform task cost              | `StaticPool`         | `ErrGroup`           | -27 % |
| Uniform, undersized cap (= 8)  | `StaticPool`         | `RoundRobinPool`     | -23 % |
| Uniform, oversized cap (≥ 32)  | tied                 | tied                 | < 2 % |
| Variable task cost             | `RoundRobinPool`     | `StaticPool`         | -22 % |
| Pinned workers                 | `StaticPool` (still) | `RoundRobinPool`     | -22 % |

**There is no universal winner.** Pool choice depends on the
workload:

- **Uniform task cost** → `StaticPool` is best. A shared channel +
  any-worker dispatch + zero-alloc submit beats every alternative.
- **Variable task cost** → `RoundRobinPool` is best. Per-worker
  channels avoid lock contention; the static-dispatch imbalance
  matters less than the saved sync overhead.
- **Mid-range** → `ErrGroup` is consistently the best of the
  closure-based pools.

## Notes on benchmark methodology

- A single suite invocation (`go test -bench=.`) is *unreliable* for
  comparing pools against each other. The same `StaticPool` code
  measured at 48, 52, and 71 ms across three different harness
  arrangements. Always isolate.
- The cap-sweep benches (Case 1) use one bench function with five
  `b.Run` subtests; subtests within one parent don't get individual
  cooldowns. Within-pool sweep numbers are comparable; absolute
  numbers vs Case 0 are slightly off because of this.
- The cap-sweep code also uses method-value indirection
  (`submit := p.Go; submit(...)`). A separate factorial micro-bench
  earlier showed this can shift absolute timings by ~30 % vs direct
  method calls under suite-style measurement, but in fully-isolated
  runs the effect is partially masked by other steady-state
  differences. Treat absolute cap-sweep numbers with caution; the
  within-table *shape* (cap-vs-time curve) is reliable.

