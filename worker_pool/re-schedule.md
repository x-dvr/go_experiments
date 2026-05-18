The traces in `trace.md` attribute big chunks of *sched-delay* (and
the matching sync-delay) to `runtime.chansend1` and
`runtime.chanrecv2`. These are two distinct measurements that share
the same call site:

- **sync profile** = time the goroutine is in **waiting** state
  (parked, off CPU completely). Attributed to where it was when it
  parked.
- **sched profile** = time the goroutine is **runnable** but waiting
  for a P (ready, but not yet running). Attributed to the stack at
  the moment it became runnable.

Both numbers can point at `chansend1` / `chanrecv2` because the
goroutine *parked* there and *re-entered the runqueue* from there.
The full life-cycle:

```
running ── chan op needs to wait ──▶ gopark ──▶ waiting   [sync delay starts]
waiting ── counterparty arrives ──▶ goready  ──▶ runnable  [sync delay ends, sched delay starts]
runnable ── scheduler picks it ───▶ execute             [sched delay ends]
```

**Direct hand-off wakes the counterparty.** When `chansend` finds
   a *waiting* receiver, it copies the value directly into the
   receiver's frame and calls `goready(receiver)`. The receiver
   becomes runnable from inside `chanrecv2`, which is exactly the
   stack the sched profile attributes time to. So a send on one
   goroutine can produce sched-delay on the *other* goroutine's
   chanrecv stack — that's why workers show up as "waiting in
   chanrecv2".

- **RR submitter** (14 ms sync + 616 ms sched in `chansend1`): the
  submitter occasionally blocks on a full per-worker buffer (14 ms
  parked), and every time it unblocks it queues again before getting
  CPU (616 ms cumulative across ~hundreds of wake-ups).
- **Static submitter** (1 ms sync + 713 ms sched): rarely blocks on
  the channel, but its wake-ups happen *more often* because the
  shared channel drains faster and the submitter keeps cycling
  between "ready" and "running". Higher sched-delay, lower
  sync-delay, faster wall-clock.

## Why does NoPool show 5.77 s of async preempt if each goroutine runs < 5 µs?

Each `workHard(1e5)` call uses ~5 µs of CPU, which is three orders
of magnitude below the 10 ms "long-running goroutine" threshold that
`sysmon` uses to trigger async preemption. So where do the 5.77 s of
cumulative `runtime.asyncPreempt2` time come from in NoPool's sched
profile?

The 10 ms / `sysmon` rule is **not** the only trigger for async
preemption. The same `asyncPreempt` / `asyncPreempt2` machinery is
also used by:

1. **GC stop-the-world transitions.** Each GC cycle has two STW
   phases (sweep-termination → mark and mark-termination → sweep).
   To enter STW, the runtime sends an async-preempt signal to every
   goroutine currently running on a P, forcing them to park at the
   next safe point. With 10 000 short goroutines, the runtime is
   constantly entering/leaving these phases, and each transition
   preempts whichever ~16 goroutines are running across the P's at
   that moment.

2. **GC mark-assist forced yields.** When a goroutine allocates
   during the GC mark phase and is behind on its allocation budget,
   the runtime makes it "assist" the GC by marking some objects.
   The transition into assist mode is itself a preemption point —
   the goroutine is forced into the runtime, queued, and returned to
   the runqueue once it has assisted enough. In NoPool, every task
   closure allocates (the `func() { sink[i] = workHard(...) }` body
   captures `i` and escapes), so every goroutine is a potential
   mark-assist target.

3. **Stack growth / shrink.** New goroutines start with a 2 KB stack.
   When a function call would overflow it, the runtime grows the
   stack (which is essentially a preempt + remap + resume). NoPool
   creates a fresh goroutine for every task, so each one pays a
   stack-growth check on its first `workHard` call. Stack shrink
   during GC scan is also a preempt-resume.

The trace itself corroborates GC as the dominant cause: NoPool's
sched profile lists `runtime.gcBgMarkWorker` (157 ms cum) and
`runtime.bgsweep` (52 ms) right next to the 5.77 s of
`asyncPreempt2` — i.e. the GC machinery is active and running mark
workers, which is what's preempting everything else.

In other words: it's not that *any one* goroutine ran for 10 ms;
it's that 10 000 short goroutines collectively cause the runtime to
hit GC and STW transitions many times during the workload, and each
transition forcibly preempts the ~16 goroutines that happen to be
running at that moment. With 10 k allocations driving multiple GC
cycles, the total preemption time accumulates fast.

### Where to read about this

- `src/runtime/preempt.go` — async-preemption implementation; see
  the package comment for the signal-based protocol and which
  conditions trigger it.
- `src/runtime/mgc.go` — `gcStart`, `gcMarkDone`, `stopTheWorld`,
  `startTheWorld`. The STW entry calls `preemptone` on every running
  goroutine.
- `src/runtime/mgcmark.go` — `gcAssistAlloc` (the mark-assist path
  that forces an allocator to help GC before continuing).
- `src/runtime/proc.go` — `sysmon` (the 10 ms "long-running" check),
  `preemptone`, `suspendG`.
- Go release notes for 1.14, "Goroutine preemption" — the original
  introduction of signal-based async preemption and the rationale.
- Austin Clements, *"Proposal: Non-cooperative goroutine preemption"*
  ([github.com/golang/proposal/blob/master/design/24543-non-cooperative-preemption.md](https://github.com/golang/proposal/blob/master/design/24543-non-cooperative-preemption.md))
  — the design doc for async preemption, including its use by GC.
- `src/runtime/chan.go` — `chansend`, `chanrecv`, the
  parking/unparking calls. Worth reading top-to-bottom; it's
  well-commented.
- `src/runtime/proc.go` — `gopark`, `goready`, `goschedImpl`,
  `runqput`, `findRunnable`. This is the scheduler proper.
- `src/runtime/HACKING.md` — terminology (G / M / P), invariants,
  design notes.
- `src/runtime/trace/trace.go` and `src/cmd/trace/` — definitions of
  the events that `runtime/trace` records and how the pprof slicing
  works.

Talks and writeups:

- Dmitry Vyukov, *"Scalable Go Scheduler Design Doc"* — the design
  doc that introduced the work-stealing scheduler
  ([golang.org/s/go11sched](https://golang.org/s/go11sched)).
- Kavya Joshi, *"The Scheduler Saga"* (GopherCon 2018) — clearest
  talk on G / M / P + parking / unparking.
- Madhav Jivrajani, *"Demystifying channels in Go"* — walks through
  `chansend` / `chanrecv` line by line.
- William Kennedy's *"Scheduling in Go"* series (ardanlabs.com) —
  three parts on the runtime scheduler.
- Russ Cox, *"Go's work-stealing scheduler"* notes in his blog
  series.

