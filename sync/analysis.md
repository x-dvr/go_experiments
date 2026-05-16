# Sync benchmark analysis (v2 — after fixes)

This is the post-fix version of the earlier analysis. The data race in
`cond.go` is fixed; the racy buffer hand-off is replaced by proper
ping-pong; `PoolChan`'s allocation overhead is fixed; the dead-code
broken `buffer.go` is deleted.

## What changed

1. **`cond.go` — `done` is now `atomic.Bool`**, eliminating the cross-
   lock race the race detector caught on every run.
2. **`cond.go` — producer waits for `ready1 == false` before writing
   `buf1`** (and likewise for `buf2`). The producer no longer races
   the consumer mid-hash; `-race` is clean.
3. **`cond.go` — `Wait` predicate widened to `!ready && !done`** so a
   consumer that wakes from `done = true` without `ready` set still
   exits cleanly. The old code relied on `setDone` setting `ready =
   true`, which was misleading — there was no fresh data.
4. **`pool.go` — switched from `*bytes.Buffer` + `io.CopyN` to
   `*[]byte` + `io.ReadFull`.** `io.CopyN` wraps the source in a
   `*io.LimitedReader` that escapes to the heap once per call — 1 000
   escapes per `Start` was *all* of `PoolChan`'s alloc count. With
   the direct `io.ReadFull(f, *buf)`, allocs/op drops from **1 009 to
   9** and B/op drops from 70 KiB to 18 KiB.
5. **`sync_test.go` — pooled buffers are now `Put` back** after the
   test reads them.
6. **`buffer.go` deleted.** It was dead code with a broken `sync.Cond`
   pattern (no predicate loop, no flag) and was used by nothing.
7. **`ring.go` — `done` propagation cleaned up.** Removed the empty-
   critical-section "memory fence" pattern; `atomic.Bool` provides
   the needed ordering. Documented the leading-edge alignment
   limitation in a comment.

## Results

`go test -bench=. -benchtime=3s -count=6` on Intel i7-10870H @ 2.20 GHz,
Linux amd64, Go 1.25, aggregated with `benchstat`:

| Bench       | sec/op        | B/op            | allocs/op |
|-------------|---------------|-----------------|----------:|
| `Seq`       | 316.3 ms ± 3% |     120 B       |      3    |
| `Chan`      | 178.0 ms ± 2% |    62.50 MiB    |  1,010    |
| `PoolChan`  | 174.1 ms ± 2% |    18.44 KiB ±19% |    9    |
| `Cond`      | 173.7 ms ± 1% |     546.5 B ±34%|      8    |
| `Ring`      | 173.1 ms ± 2% |     152 B ±11%  |      4    |

All four parallel variants are race-clean (`go test -race -bench=.`).

## Interpretation

### Why Seq is 1.8× slower than the parallel variants — unchanged

The two stages still split cleanly:

| Stage           | Rate (this machine) | 64 MiB takes |
|-----------------|---------------------|--------------|
| /dev/urandom    | ~400 MB/s           | ~160 ms      |
| SHA-256         | ~460 MB/s           | ~140 ms      |

- Seq runs them in series: 160 + 140 ≈ 316 ms. ✓
- Parallel variants overlap: `max(160, 140) ≈ 173–178 ms`. ✓

### Why the four parallel variants are clustered at ~175 ms

The bottleneck is still **/dev/urandom**, not the synchronisation
primitive. Whether the producer hands buffers off through a channel,
a Cond ping-pong, or a ring, it spends most of its life sleeping in
`read(2)`. The differences in synchronisation overhead would need to
be > 10 ms / 1000 tasks = 10 µs/task to show through — and the actual
overhead per task is < 1 µs.

**This benchmark does not, despite its name, measure synchronisation
primitives.** It measures urandom throughput, with sync primitives as
a sidecar. To make it a meaningful sync comparison, swap urandom for
a CPU-bound source (a ChaCha20 PRNG, or `crypto/rand` with a fast
backend) at hundreds of MB/s so the producer becomes CPU-bound and
the synchronisation cost can dominate.

### `PoolChan`: 9 allocs instead of 1 009

The fix to `pool.go` is the biggest win. Old code:

```go
buf := p.P.Get().(*bytes.Buffer)
io.CopyN(buf, f, S64k)  // escapes a *io.LimitedReader per call
```

New code:

```go
buf := p.P.Get().(*[]byte)
io.ReadFull(f, *buf)  // no helper allocation
```

The pool now lives up to its promise: B/op drops 3,800× (62.5 MiB →
18 KiB) compared to plain `Chan`. The 9 remaining allocs are the
fixed scaffolding cost — 2 channels, 2 goroutines, the open `os.File`,
a `*[]byte` returned from `Get` when the pool is first empty, etc.

The wall-clock improvement of `PoolChan` over `Chan` is tiny (4 ms,
~2 %), because both are urandom-bound and Go's allocator handles
single-threaded 64 KiB allocations very efficiently. The pool would
matter more under contention or with a faster source.

### `Cond` after the race fixes

`Cond` was the previous run's "fast" parallel variant (~165 ms in the
original numbers). After adding the `for p.ready1 { Wait() }` barrier
on the producer side, it runs at 173.7 ms — about 5 % slower. That's
the cost of correctness: the producer now blocks until the consumer
finishes hashing before overwriting a buffer, instead of plowing
ahead and (sometimes) corrupting data mid-hash.

`Ring` (173.1 ms) is now slightly faster than `Cond`, which is the
expected ranking — the ring's per-slot ownership is cleaner than
ping-pong with two Conds and two consumers.

## Suggested next experiments

1. **Replace `/dev/urandom` with a CPU-bound stream.** This is the
   single most impactful change to make this an actual sync
   benchmark. Try `math/rand/v2`'s ChaCha8 source (which can do
   1+ GB/s) and the ranking will change.
2. **Add asymmetric workload variants.** A version where SHA-256 is
   replaced with a 10× more expensive hash would let the consumer
   become the bottleneck and expose differences in back-pressure
   behaviour between the variants.
3. **Test with `GOGC=off`.** The 62.5 MiB of allocation in `Chan` is
   currently absorbed by the GC, but you'd see PoolChan's allocation
   advantage manifest as wall-clock if GC pressure rose (e.g. by
   running multiple Chan instances in parallel).
