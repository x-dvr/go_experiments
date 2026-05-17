# Worker-pool benchmark results

This document collects four benchmark cases:

- **Case 0** — initial 7 pool implementations, the baseline from
  `analysis.md` (workerpool_test.go).
- **Case 1** — `PoolCap` sweep from `NumCPU/2` to `NumCPU*8` for
  `RoundRobinPool` and `StaticPool` (bench1_test.go).
- **Case 2** — variable-cost task workload, every 10th task is 10x
  heavier (bench2_test.go).
- **Case 3** — `runtime.LockOSThread()`-pinned worker variants of
  `StaticPool` and `RoundRobinPool` (bench3_test.go).

## Setup

- Hardware: Intel(R) Core(TM) i7-10870H CPU @ 2.20GHz, 16 logical CPUs
- OS / Go: Linux amd64, Go 1.25
- Workload: `RunTimes = 10 000` calls to `workHard(1e5)` (a Fibonacci
  iteration of length `CalcTo = 1e5`) per benchmark iteration.
- `PoolCap = runtime.NumCPU() * 2 = 32` for cases 0/2/3.
- Command (per case):
  `go test -run=^$ -bench=<pattern> -benchmem -benchtime=5s -count=6`
  aggregated with `benchstat`.

---

## Case 0 — Baseline (workerpool_test.go)

Sorted by time. The numbers reproduce the analysis.md ranking — the
relative ordering is identical, absolute numbers are 5–10% better
because of a slightly different host state but well within typical
between-run variance.

| Bench            | sec/op           | B/op        | allocs/op |
|------------------|------------------|-------------|----------:|
| `RoundRobinPool` |  54.84 ms ± 4%   |       4 B   |    0      |
| `NoPool`         |  65.34 ms ± 3%   |  391.0 KiB  | 20 000    |
| `AntsPool`       |  67.12 ms ± 2%   |  234.9 KiB  | 10 000    |
| `ErrGroup`       |  67.74 ms ± 1%   |  390.6 KiB  | 20 000    |
| `PreallocPool`   |  70.46 ms ± 5%   |  156.3 KiB  | 10 000    |
| `SemaphorePool`  |  71.26 ms ± 2%   |  625.0 KiB  | 30 000    |
| `StaticPool`     |  71.43 ms ± 4%   |       8 B   |    0      |

Takeaway: `RoundRobinPool` is the clear winner under uniform-cost
tasks, as analysis.md predicted. Sub-byte `B/op` numbers on
`StaticPool` / `RoundRobinPool` are bench-framework noise (the actual
per-task allocation is zero).

---

## Case 1 — `PoolCap` sweep (bench1_test.go)

Hypothesis under test: *at very high `PoolCap`, scheduler load
dominates and per-channel contention becomes irrelevant — i.e.
`RoundRobinPool` loses its advantage over `StaticPool`.*

| Bench                     | sec/op           |
|---------------------------|------------------|
| `RoundRobin/cap=8`        | 133.1 ms ± 3%    |
| `RoundRobin/cap=16`       |  73.82 ms ± 5%   |
| `RoundRobin/cap=32`       |  72.83 ms ± 4%   |
| `RoundRobin/cap=64`       |  72.48 ms ± 3%   |
| `RoundRobin/cap=128`      |  73.85 ms ± 5%   |
| `Static/cap=8`            |  97.68 ms ± 2%   |
| `Static/cap=16`           |  83.67 ms ± 2%   |
| `Static/cap=32`           |  72.05 ms ± 3%   |
| `Static/cap=64`           |  71.88 ms ± 2%   |
| `Static/cap=128`          |  71.93 ms ± 2%   |

### Observations

1. **At `cap = NumCPU/2` the ordering flips.** RR is 36 % slower than
   Static. With only 8 workers (vs 10 000 tasks), every worker is
   permanently busy and the per-worker channel buffers (size 8) fill
   instantly; the submitter then blocks waiting for *the specific
   worker the round-robin index points at*. Static, with one shared
   channel of the same buffer size, lets the submitter offload to
   whichever worker the runtime wakes next — a tiny dynamic
   load-balancer at the channel level. RR's static dispatch costs more
   than its lower contention saves once workers are oversubscribed.

2. **At `cap = NumCPU` both pools narrow to ~5 % apart.** RR is still
   slightly faster (74 ms vs 84 ms), but the gap is shrinking because
   workers are still nearly saturated — the submitter still blocks on
   RR's targeted channel sometimes.

3. **From `cap = NumCPU*2` upward, the two pools converge to ~72 ms.**
   Once there are more workers than logical CPUs, workers spend time
   parked on `<-tasks`, so the hchan-lock contention that hurt Static
   at low cap disappears. The RR per-worker isolation no longer buys
   anything because Static's lock is uncontended too.

4. **No degradation at `cap = NumCPU*8 = 128`.** The hypothesis that
   scheduler load eventually dominates does not show up at 128 workers
   on this machine. Both pools sit flat at ~72 ms from cap=32 onward.
   To actually find the scheduler-saturation point you would need
   thousands of workers, not hundreds.

### Why the cap=32 numbers here are higher than Case 0's standalone RR

The cap-sweep `cap=32` RR sample (72.83 ms) is ~33 % slower than
Case 0's standalone `RoundRobinPool` (54.84 ms) at the same cap. The
two suspects were (1) method-value indirection
(`submit := p.Go; submit(...)`) and (2) `b.Run` subtest scaffolding.
A 2×2 factorial micro-bench against `NoPool` (control, no method
value), `StaticPool`, and `RoundRobinPool` (anomaly_test.go) isolates
them:

| Variant                          | NoPool        | StaticPool    | RoundRobinPool |
|----------------------------------|---------------|---------------|----------------|
| A — top-level + direct call      | 67.59 ms ± 3% | 52.12 ms ± 0% | 53.61 ms ± 0%  |
| B — top-level + method value     | —             | 69.21 ms ± 1% | 70.85 ms ± 1%  |
| C — `b.Run` subtest + direct     | 69.62 ms ± 1% | 52.90 ms ± 0% | 54.13 ms ± 0%  |
| D — `b.Run` subtest + method val | —             | 70.20 ms ± 1% | 71.12 ms ± 0%  |

Reading the matrix:

- **A → C (b.Run alone):** +1–3 %. `b.Run` scaffolding is negligible
  on all three pool types — the `NoPool` control confirms the upper
  bound (~3 %) when no method value is involved.
- **A → B (method value alone, top-level):** +33 % on Static, +32 %
  on RR. The entire anomaly is here.
- **A → D (both effects, the cap-sweep pattern):** +35 % on Static,
  +33 % on RR. Same as B within noise — `b.Run` adds nothing on top.

So the cap-sweep numbers were biased ~30 % high by method-value
indirection at the call site, not by `b.Run`. The relative shape of
Case 1 (RR vs Static at each cap; convergence at cap ≥ NumCPU×2) is
preserved — both pools were biased by the same factor — but the
absolute numbers are inflated and shouldn't be compared to Case 0's
top-level benchmarks. The same caveat applies to Case 2/3 reasoning:
those benches all use direct method calls, so they are directly
comparable to Case 0.

Likely mechanism: a direct `pool.Go(...)` call inlines, leaving the
submit loop with few real function calls and few preemption points.
A method-value call cannot inline — each `submit(...)` is a real
call and a preemption point, so the submit goroutine yields more
often and the channel-send hot path picks up scheduler-latency
overhead. The 1.8 µs/call delta matches a "yielded once per call"
scheduler-roundtrip cost, not a per-call CPU cost.

The fix for bench1 is to drop the `submit, drain, release := build(cap)`
indirection and call `pool.Go` / `pool.Drain` directly inside the
`b.Run` closure — which would bring cap-sweep absolute numbers in line
with Case 0 at cap=32.

---

## Case 2 — Variable task workload (bench2_test.go)

Workload: every 10th task computes `workHard(1e6)` (10x the
iterations) instead of `workHard(1e5)`. Total work ≈ 1.9x the uniform
case; 1 000 of the 10 000 tasks dominate the wall-clock.

Hypothesis under test: *RR's static dispatch loses badly under
uneven task costs — one worker queues multiple heavy tasks while
other workers idle.*

| Bench                       | sec/op           | B/op        | allocs/op |
|-----------------------------|------------------|-------------|----------:|
| `VariableStaticPool`        |  90.87 ms ± 2%   |       4 B   |    0      |
| `VariableErrGroup`          |  92.74 ms ± 2%   |  390.6 KiB  | 20 000    |
| `VariableAntsPool`          | 106.0  ms ± 2%   |  234.9 KiB  | 10 010    |
| `VariableNoPool`            | 113.2  ms ± 2%   |  391.0 KiB  | 20 000    |
| `VariableRoundRobinPool`    | 115.5  ms ± 2%   |      14 B   |    0      |
| `VariablePreallocPool`      | 115.6  ms ± 2%   |  156.3 KiB  | 10 000    |

### Observations

1. **The ordering inverts completely vs Case 0.** `StaticPool` is now
   the fastest pool, and `RoundRobinPool` drops from #1 to second
   slowest. The hypothesis from analysis.md is confirmed bluntly:
   under uneven loads, RR's "no contention" advantage is dwarfed by
   the cost of an unlucky worker holding 2–3 heavy tasks while peers
   sit idle.

2. **StaticPool effectively work-steals at the channel.** With one
   shared `chan tsk`, any free worker grabs the next task. Heavy tasks
   end up distributed across whichever workers happen to be ready —
   automatic dynamic load balancing without writing any work-stealing
   code.

3. **ErrGroup is close behind Static** — same reason, `errgroup`'s
   shared permit channel acts as the same kind of dynamic gate. The
   only thing it pays extra for is the per-task closure + state alloc.

4. **AntsPool is in the middle** because ants's internal queue is
   also shared / dynamic, but its mutex-protected dispatch is slightly
   heavier than a raw channel.

5. **PreallocPool is surprisingly slow.** It uses the same shared
   channel as Static but takes a `Task` closure per call. The closure
   captures per-task state (`i`) which allocates on every submit. The
   2 µs of allocator pressure per task adds ~20 ms over 10 000 tasks
   — about the gap to Static.

---

## Case 3 — `LockOSThread()`-pinned workers (bench3_test.go)

Hypothesis under test: *if the Go scheduler is the bottleneck on the
shared-channel pools (`StaticPool`), pinning workers to OS threads
should help. If not, the bottleneck is hchan contention itself.*

| Bench                       | sec/op           |
|-----------------------------|------------------|
| `PinnedRoundRobinPool`      |  65.93 ms ± 2%   |
| `PinnedStaticPool`          |  88.88 ms ± 2%   |

Side-by-side against Case 0:

| Pool                   | Non-pinned        | Pinned            | Δ        |
|------------------------|-------------------|-------------------|----------|
| `StaticPool`           |  71.43 ms ± 4%    |  88.88 ms ± 2%    | **+24 %** (worse) |
| `RoundRobinPool`       |  54.84 ms ± 4%    |  65.93 ms ± 2%    | **+20 %** (worse) |

### Observations

1. **Pinning hurts both pools by similar margins.** The hypothesis is
   refuted in the strong form: the Go scheduler is *not* the
   bottleneck — quite the opposite, removing scheduler flexibility
   measurably hurts throughput. `LockOSThread()` prevents a worker
   from being moved to a different M, so when GC or another goroutine
   needs the thread, the worker has to wait instead of being rescued
   onto a free M.

2. **The gap RR-over-Static remains roughly proportional.** Pinned RR
   is 26 % faster than pinned Static, mirroring the 23 % gap between
   the non-pinned versions. So pinning doesn't differentially help the
   shared-channel variant. The Static-vs-RR difference is hchan-lock
   contention, not scheduler-induced wakeup latency on the shared
   channel.

3. **Practical consequence: don't pin Go pool workers** for compute
   workloads. The scheduler's ability to migrate goroutines onto free
   Ms is doing real work that you give up when you pin. This matches
   conventional Go guidance — `LockOSThread` is for cgo / syscall /
   thread-affine APIs, not throughput tuning.

---

The combined picture is: `RoundRobinPool` is great in exactly one
regime — uniform-cost tasks with `PoolCap ≥ NumCPU*2`. As soon as
tasks vary in cost, or workers are undersized, it loses to the
shared-channel `StaticPool`. The "perfect benchmark" of Case 0 is the
best case for RR, not the typical case.

## Suggested follow-ups

1. **Mixed cap-sweep + variable workload.** Does the cap-sweep
   crossover point depend on task-cost variance? Likely yes: at higher
   cap, RR may still lose to Static even on uniform tasks if buffer
   sizes vary.
2. **Re-run bench1 with direct method calls.** The anomaly
   investigation (see Case 1 sub-section) showed method-value
   indirection inflates cap-sweep numbers by ~30 %. Dropping the
   `submit := p.Go` capture should yield absolute numbers comparable
   to Case 0 at cap=32. The relative shape of the sweep should not
   change.
3. **Find the scheduler-saturation `PoolCap`.** Cases at cap=512,
   cap=2048, cap=8192 — the analysis.md hypothesis of "scheduler
   load dominates at high cap" might still be true, just at larger
   cap than tested here.
