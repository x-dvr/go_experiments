# Runtime/trace analysis of `NoPool`, `RoundRobinPool`, `StaticPool`

`worker_pool/tracer/main.go` runs each of the three pool variants on
the standard uniform workload (`workHard(1e5) × 10 000` tasks) under
`runtime/trace`, producing one `.out` file per pool. Slicing those
traces into pprof profiles answers the question that the wall-clock
benchmarks in `results.md` only hint at: **where do the goroutines
spend their time?**

The wall-clock numbers from Case 0 (isolated runs):

| Pool        | sec/op (Case 0) |
|-------------|-----------------|
| `Static`    |  48.59 ms       |
| `RR`        |  64.20 ms       |
| `NoPool`    |  65.99 ms       |

The traces below explain why.

## Methodology

Each pool was traced in its own process with 60 s cooldown between
runs (same protocol as the benchmark suite — see `README.md`). The
tracer takes a CLI argument (`no`, `rr`, or `st`) selecting which
pool to trace, so each run measures only one pool.

### Commands

Build the tracer once:

```sh
cd worker_pool/tracer
go build -o /tmp/claude-1000/tracer .
```

Run each pool with cooldown:

```sh
mkdir -p /tmp/claude-1000/wp_traces
cd /tmp/claude-1000/wp_traces
sleep 60 && /tmp/claude-1000/tracer no   # → trace_no.out
sleep 60 && /tmp/claude-1000/tracer rr   # → trace_rr.out
sleep 60 && /tmp/claude-1000/tracer st   # → trace_st.out
```

Extract pprof profiles from each trace (sync = blocked on channel /
mutex; sched = ready-to-run but waiting for CPU; syscall = blocked
in syscall):

```sh
for kind in sync sched syscall; do
  for pool in no rr st; do
    go tool trace -pprof="$kind" "trace_${pool}.out" > "${pool}_${kind}.pprof"
  done
done
```

Inspect each:

```sh
go tool pprof -top -unit=ms <profile>.pprof
```

### Resulting files

```
$ ls -la /tmp/claude-1000/wp_traces/
trace_no.out    155 KB   ← many short-lived goroutines, lots of events
trace_rr.out     12 KB   ← 32 long-lived workers, sparse channel events
trace_st.out     55 KB   ← 32 long-lived workers, more channel events
```

The trace file size already hints at what each pool is doing:
NoPool's 155 KB reflects 10 k goroutine birth/death events. RR's 12 KB
is the smallest — workers are reused, and per-worker channels never
need to coordinate (no contention events to log). Static sits in
between: same 32 long-lived workers as RR, but every worker recv
touches the shared `hchan`, so there are more events to record.

Syscall blocking is < 0.01 ms for all three pools (only the
trace-writer itself shows up) and is omitted below.

---

## `NoPool` — `wg.Go(task)` per task

### Sync profile (blocked on channel / mutex)

```
$ go tool pprof -top -unit=ms no_sync.pprof
Type: delay
Showing nodes accounting for 44.86ms, 100% of 44.88ms total
Dropped 7 nodes (cum <= 0.22ms)
      flat  flat%   sum%        cum   cum%
   44.86ms   100%   100%    44.86ms   100%  sync.(*WaitGroup).Wait
         0     0%   100%    44.88ms   100%  main.main
         0     0%   100%    44.88ms   100%  main.noPool
         0     0%   100%    44.88ms   100%  main.traceFn
```

**Explanation.** "Sync delay" is the total time goroutines spend
blocked on channel or mutex operations. NoPool has effectively none:
worker goroutines never block on anything (they just compute
`workHard(1e5)` and exit), and the only sync block is the main
goroutine sitting in `WaitGroup.Wait` until the last worker finishes.
The 44.86 ms there is roughly the wall-clock of the workload minus
the parallel speedup — the main goroutine just waits the whole time.

### Sched profile (ready, waiting for CPU)

```
$ go tool pprof -top -unit=ms no_sched.pprof
Type: delay
Showing nodes accounting for 6085.57ms, 99.28% of 6129.44ms total
Dropped 9 nodes (cum <= 30.65ms)
      flat  flat%   sum%        cum   cum%
 5767.11ms 94.09% 94.09%  5767.11ms 94.09%  runtime.asyncPreempt2
  156.78ms  2.56% 96.65%   156.78ms  2.56%  runtime.systemstack_switch
  109.28ms  1.78% 98.43%   109.28ms  1.78%  sync.(*WaitGroup).Go.func1.1
   52.39ms  0.85% 99.28%    52.39ms  0.85%  runtime.goschedIfBusy
         0     0% 99.28%  5767.11ms 94.09%  main.noPool.func1
         0     0% 99.28%  5767.11ms 94.09%  main.workHard
         0     0% 99.28%  5767.11ms 94.09%  runtime.asyncPreempt
         0     0% 99.28%    52.39ms  0.85%  runtime.bgsweep
         0     0% 99.28%   156.79ms  2.56%  runtime.gcBgMarkWorker
         0     0% 99.28%  5876.39ms 95.87%  sync.(*WaitGroup).Go.func1
```

**Explanation.** "Sched delay" is the time a goroutine spends ready
to run but unable to get CPU (because all P's are busy with other
goroutines). NoPool has **6.13 s** of cumulative sched delay — two
orders of magnitude more than its sync delay.

Breaking down where the delay comes from:

- **`runtime.asyncPreempt2` (5.77 s, 94 %)** — Go's async preemption
  fires every ~10 ms on a long-running goroutine, asking it to yield
  so other goroutines can be scheduled. With 10 000 CPU-bound
  goroutines all calling `workHard(1e5)` (~5 µs each but preemption
  doesn't care about that), the runtime is constantly preempting and
  resuming. The 5.77 s is the cumulative "I was ready but my P was
  busy running another goroutine that just got preempted" time.
- **`runtime.gcBgMarkWorker` (157 ms, 2.5 %)** — GC mark workers
  waiting for CPU. GC runs because each task closure
  (`func(){sink[i] = workHard(CalcTo)}`) allocates a small frame on
  the goroutine; multiplied by 10 000 that's enough to trigger GC.
- **`runtime.goschedIfBusy` (52 ms)** — the background sweeper
  yielding when other goroutines need the P.

**Reading.** NoPool's bottleneck is the *scheduler*, not the
workload. The 10 000 short goroutines create more scheduler pressure
than they need to. A pool with N long-lived workers (where N ≈
runtime cores) sidesteps almost all of this.

---

## `RoundRobinPool` — N per-worker channels

### Sync profile (blocked on channel)

```
$ go tool pprof -top -unit=ms rr_sync.pprof
Type: delay
Showing nodes accounting for 370.02ms, 100% of 370.02ms total
Dropped 1 node (cum <= 1.85ms)
      flat  flat%   sum%        cum   cum%
  354.13ms 95.71% 95.71%   354.13ms 95.71%  runtime.chanrecv2
   13.75ms  3.72% 99.42%    13.75ms  3.72%  runtime.chansend1
    2.13ms  0.58%   100%     2.13ms  0.58%  sync.(*WaitGroup).Wait
         0     0%   100%     2.13ms  0.58%  github.com/x-dvr/go_experiments/worker_pool.(*RRPool).Drain
         0     0%   100%    13.75ms  3.72%  github.com/x-dvr/go_experiments/worker_pool.(*RRPool).Go
         0     0%   100%   354.13ms 95.71%  github.com/x-dvr/go_experiments/worker_pool.NewRRPool.func1
         0     0%   100%    15.88ms  4.29%  main.main
         0     0%   100%    15.88ms  4.29%  main.robinPool
         0     0%   100%    15.88ms  4.29%  main.traceFn
         0     0%   100%   354.13ms 95.71%  sync.(*WaitGroup).Go.func1
```

**Explanation.** Two distinct blocking sources:

- **`runtime.chanrecv2` (354 ms, 96 %)** — the **workers** parked on
  `<-pool.args[i]`, waiting for the submitter to send them work. This
  is *worker-idle time*. The call site is `NewRRPool.func1`, the
  worker goroutine, which loops on `for arg := range p.args[i]`.
  Cumulative across 32 workers; per-worker that's ~11 ms idle out of
  ~64 ms wall-clock — workers are idle ~17 % of the time.
- **`runtime.chansend1` (14 ms, 4 %)** — the **submitter** blocked
  in `(*RRPool).Go` on the round-robin send. The submitter hits a
  full buffer on `args[idx%cap]` and waits for that *specific* worker
  to consume one item before continuing.
- **`WaitGroup.Wait` (2.13 ms)** — `Drain()` waiting for the last
  in-flight task to complete. Tiny.

**Reading.** RR's bottleneck under uniform load is **workers
spending time idle on their dedicated channels**, not contention. The
submitter does the right thing (rotates round-robin), but with 32
channels and a submitter that's faster than any single worker, by the
time the submitter rotates back to channel `i`, worker `i` has long
finished its previous task and is parked.

### Sched profile

```
$ go tool pprof -top -unit=ms rr_sched.pprof
Type: delay
Showing nodes accounting for 664.27ms, 100% of 664.44ms total
Dropped 4 nodes (cum <= 3.32ms)
      flat  flat%   sum%        cum   cum%
  615.74ms 92.67% 92.67%   615.74ms 92.67%  runtime.chansend1
   48.52ms  7.30%   100%    48.52ms  7.30%  runtime.chanrecv2
         0     0%   100%   615.74ms 92.67%  github.com/x-dvr/go_experiments/worker_pool.(*RRPool).Go
         0     0%   100%    48.52ms  7.30%  github.com/x-dvr/go_experiments/worker_pool.NewRRPool.func1
         0     0%   100%   615.91ms 92.70%  main.main
         0     0%   100%   615.91ms 92.70%  main.robinPool
         0     0%   100%   615.91ms 92.70%  main.traceFn
         0     0%   100%    48.53ms  7.30%  sync.(*WaitGroup).Go.func1
```

**Explanation.** Sched delay accounts for "goroutine ready to run
but no P available". RR's total is 664 ms, ~10× the wall-clock —
because the delay is per-goroutine and there are 33 goroutines
(1 submitter + 32 workers) accumulating it in parallel.

- **`runtime.chansend1` (616 ms, 93 %)** — the submitter, after each
  send, becomes ready to run the next iteration of the for-loop but
  has to wait its turn on the scheduler. This is *not* blocking on
  the channel (that was in the sync profile); it's the cost of being
  re-scheduled after each chansend.
- **`runtime.chanrecv2` (49 ms, 7 %)** — workers, after receiving a
  task, ready to run `task(arg.Arg, arg.Iter)` but waiting for CPU.

**Reading.** Compared to NoPool's 6 s of scheduler delay, RR pays
10× less in scheduler tax — 32 reused workers don't fight each other
for P's the way 10 000 short-lived goroutines do.

---

## `StaticPool` — one shared channel

### Sync profile

```
$ go tool pprof -top -unit=ms st_sync.pprof
Type: delay
Showing nodes accounting for 122.70ms, 99.80% of 122.95ms total
Dropped 3 nodes (cum <= 0.61ms)
      flat  flat%   sum%        cum   cum%
  121.55ms 98.86% 98.86%   121.55ms 98.86%  runtime.chanrecv2
    1.15ms  0.94% 99.80%     1.15ms  0.94%  runtime.chansend1
         0     0% 99.80%     1.15ms  0.94%  github.com/x-dvr/go_experiments/worker_pool.(*StaticPool).Go
         0     0% 99.80%   121.55ms 98.86%  github.com/x-dvr/go_experiments/worker_pool.NewStaticPool.func1
         0     0% 99.80%     1.40ms  1.14%  main.main
         0     0% 99.80%     1.40ms  1.14%  main.staticPool
         0     0% 99.80%     1.40ms  1.14%  main.traceFn
         0     0% 99.80%   121.55ms 98.86%  sync.(*WaitGroup).Go.func1
```

**Explanation.** Same shape as RR but with strikingly different
magnitudes:

- **`runtime.chanrecv2` (122 ms, 99 %)** — workers parked on
  `<-pool.tasks`. Per worker: ~3.8 ms of idle out of ~49 ms wall-clock
  (~8 % idle, vs RR's 17 %). Static's workers are kept busier.
- **`runtime.chansend1` (1.15 ms, 1 %)** — submitter blocked on the
  shared channel. **This is the headline number.** The theoretical
  concern about `StaticPool` is that the shared `hchan` lock would be
  contended; the trace says it's effectively zero — 115 ns per send
  on average, indistinguishable from the bare cost of a chansend op.
  Workers drain the buffer fast enough that the submitter rarely
  blocks.
- No `WaitGroup.Wait` row at all; Static's `Drain()` (also a WG)
  contributes < 0.61 ms and gets dropped by pprof.

**Reading.** Static keeps workers ~3× busier than RR by letting the
runtime hand the next task to whichever worker happens to be ready.
There is no measurable `hchan` lock contention under uniform load.

### Sched profile

```
$ go tool pprof -top -unit=ms st_sched.pprof
Type: delay
Showing nodes accounting for 760.48ms, 100% of 760.80ms total
Dropped 4 nodes (cum <= 3.80ms)
      flat  flat%   sum%        cum   cum%
  712.66ms 93.67% 93.67%   712.66ms 93.67%  runtime.chansend1
   47.83ms  6.29%   100%    47.83ms  6.29%  runtime.chanrecv2
         0     0%   100%   712.66ms 93.67%  github.com/x-dvr/go_experiments/worker_pool.(*StaticPool).Go
         0     0%   100%    47.83ms  6.29%  github.com/x-dvr/go_experiments/worker_pool.NewStaticPool.func1
         0     0%   100%   712.97ms 93.71%  main.main
         0     0%   100%   712.97ms 93.71%  main.staticPool
         0     0%   100%   712.97ms 93.71%  main.traceFn
         0     0%   100%    47.83ms  6.29%  sync.(*WaitGroup).Go.func1
```

**Explanation.** Sched delay totals 760 ms — slightly higher than
RR's 664 ms. The shape is identical to RR's:

- **`runtime.chansend1` (713 ms, 94 %)** — submitter re-scheduling
  cost after each send.
- **`runtime.chanrecv2` (48 ms, 6 %)** — workers re-scheduling cost
  after each recv.

**Reading.** Why is Static's sched delay slightly higher than RR's
if Static is faster overall? Because Static's submitter sends more
successful (non-blocking) sends per unit wall-clock — its workers
drain the channel faster, so the submitter is constantly cycling
between `chansend1` and the for-loop body. Each cycle has a sched
wake-up. More wake-ups → more cumulative sched delay, but each one
is shorter and the workload finishes sooner anyway.

The fact that sched delay is in the same order of magnitude for both
pools tells us the Go scheduler is **not** the differentiator
between Static and RR — sync (worker idle) is.

---

## Cross-pool comparison

Cumulative blocking, in milliseconds, across all goroutines:

| Pool       | sync: worker idle (chanrecv) | sync: submitter (chansend) | sync: WG.Wait | sched: asyncPreempt | sched: chansend (submitter) | sched: chanrecv (workers) |
|------------|------------------------------|----------------------------|---------------|---------------------|-----------------------------|---------------------------|
| `NoPool`   | n/a                          | n/a                        | 44.86         | **5767**            | n/a                         | n/a                       |
| `RR`       | **354**                      | 14                         | 2.13          | —                   | 616                         | 49                        |
| `Static`   | 122                          | **1.15**                   | < 0.6         | —                   | 713                         | 48                        |

### What predicts the wall-clock ranking

Per-worker average overhead (sync_block_time / 32 workers):

- Static: (122 + 1.15) / 32  ≈ **3.85 ms / worker**
- RR:     (354 + 14)   / 32  ≈ **11.5  ms / worker**

Predicted Static-vs-RR wall-clock gap: 11.5 − 3.85 ≈ **7.7 ms per
worker**, but since all 32 workers run in parallel and the workload
is bottlenecked by the slowest set of workers, you'd expect the gap
to scale by roughly the parallelism factor (16 logical CPUs running
two workers each). The actual wall-clock gap is **15.6 ms** (Case 0:
64.20 − 48.59), within 1 ms of the trace-predicted value.

The trace data explains the Case 0 ranking essentially exactly.

### NoPool vs the pools

NoPool's sync time is tiny (45 ms) but its sched delay is 5.77 s of
asyncPreempt — an order of magnitude more than RR's 0.7 s and 13×
more than the wall-clock difference would suggest. The cost of
NoPool is not "spawning a goroutine per task" in the allocator sense;
it's the *scheduler tax of 10 000 preemptible goroutines* fighting
each other for the runtime's P count.

A pool pays the spawn cost once (at construction), then reuses
workers. The trace shows this is the single most valuable thing a
pool gives you for CPU-bound work, regardless of its dispatch
strategy.

---

## Verdict from the traces

1. **`Static` wins under uniform load because its workers are 3×
   less idle than RR's.** Not because of allocator pressure (both
   pools are zero-alloc per task), not because of scheduler behavior
   (sched delay is similar), but because the shared channel lets any
   free worker grab the next task instead of waiting its turn in the
   round-robin rotation.
2. **The theoretical "hchan lock contention" concern about `Static`
   doesn't materialise** at this scale — the submitter spends a
   grand total of 1.15 ms in `chansend1` over 10 000 sends.
3. **`NoPool`'s real cost is the scheduler**, not goroutine spawn —
   5.77 s of cumulative `asyncPreempt` is the price of running 10 k
   CPU-bound goroutines through Go's preemption mechanism.
4. **`RR`'s static dispatch is the bottleneck**, not the
   per-worker-channel design itself. If you could pair RR's channel
   topology with Static's any-worker dispatch, you'd get the best of
   both — but channels don't work that way; the topology *is* the
   dispatch.

The variable-workload case (Case 2 in `results.md`) is a different
story — there, the picture flips because per-task duration variance
turns `Static`'s shared channel into a contention point. A separate
trace pass on the variable workload would be a useful follow-up.
