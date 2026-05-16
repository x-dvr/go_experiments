# Sync benchmark analysis

Five variants of a 1000 × 64KiB urandom-read → SHA-256-sum pipeline:

| Variant     | Producer / Consumer wiring                            |
|-------------|-------------------------------------------------------|
| `Seq`       | One goroutine: read then hash, in place               |
| `Chan`      | Producer goroutine → `chan []byte` size 1 → consumer  |
| `PoolChan`  | Same as Chan, buffers recycled via `sync.Pool`        |
| `Cond`      | Producer + two consumers, two `sync.Cond` ping-pong   |
| `Ring`      | Bounded ring (size 4) with head/tail atomics + 2 Cond |

## Bug analysis

### 1. `cond.go` — data race on `p.done` (confirmed by `-race`)

```
WARNING: DATA RACE
Read at  cond.go:87  by goroutine N  (consumer1: `if p.done`)
Write at cond.go:47  by goroutine M  (setDone: `p.done = true` under c2.L)
```

`setDone` writes `p.done = true` twice — once under `c1.L` (line 41),
once under `c2.L` (line 47). Consumer 1 reads `p.done` under `c1.L`;
consumer 2 reads it under `c2.L`. The two locks don't synchronise with
each other, so the write under `c2.L` races with consumer 1's read under
`c1.L`, and vice versa. **Benign** in practice (both writes set the same
`true`), but a memory-model violation — and turning off optimisations,
or running on a weaker-ordered architecture, could produce a torn read.

### 2. `cond.go` — racy buffer hand-off (theoretical, doesn't fire with these timings)

Producer:
```go
_, err := io.ReadFull(f, p.buf1[:])   // ← write OUTSIDE any lock
p.c1.L.Lock()
p.ready1 = true
p.c1.L.Unlock()
p.c1.Signal()
```

The buffer write happens-before only the **immediately following**
`c1.Unlock`, which the consumer's matching `c1.Lock` happens-after. So
iteration N's buffer write is correctly visible to iteration N's
consumer read. But the producer then writes `buf2`, releases `c2`,
loops back, and writes `buf1` **again** for iteration N+1 — *without
re-locking* `c1.L` before the write. There is no happens-before edge
between consumer's iter-N hash (inside `c1.L`) and producer's iter-N+1
buf1 write (outside any lock). If the consumer is slow enough to still
be hashing buf1(N) when the producer reaches buf1(N+1), the producer
will overwrite buf1 mid-hash and the SHA-256 digest is garbage.

The race detector did **not** flag this because at the current timings
(urandom ~160ms/64MB, hash ~140ms/64MB), the consumer always finishes
hashing buf1(N) ~150µs before the producer starts buf1(N+1). On a
slower CPU, faster urandom source, or under heavy load, the race would
fire and the result would silently be wrong. The benchmark discards the
return value (`_ = p.Start(cnt)`), so nothing notices.

The fix: make the producer wait for `ready1 == false` (consumer
finished) before writing buf1 again, i.e. proper ping-pong instead of
fire-and-forget.

### 3. `cond.go` — final `setDone` mis-flags the buffers as "fresh"

After the last successful iteration, `setDone` sets `ready1 = true` and
`ready2 = true`. There is no new data — these flags are just to wake the
consumers. The consumers wake from `Wait`, see `ready1 == true`,
**then** check `if p.done { break }`. Functional (the `done` check
catches it), but the `ready1 = true` is misleading; a future maintainer
might re-arrange the consumer code and start hashing stale data.

### 4. `ring.go` — alignment padding is approximate

```go
type RingProducer struct {
    head     atomic.Uint64  // 8B
    _pad0    [56]byte       //  → 64B total
    tail     atomic.Uint64  // 8B
    _pad1    [56]byte       //  → 64B total
    done     atomic.Bool    // 1B
    _pad2    [63]byte       //  → 64B total
    slots    []RingSlot
    ...
}
```

The byte counts add up to 64. **But** the struct itself isn't
guaranteed to start on a 64B boundary — Go's allocator gives 8B
alignment for most heap objects. If the allocation lands at, say,
offset 8 of a cache line, `head` shares its line with whatever
precedes the struct in memory. The padding *between* `head` and `tail`
still works (head and tail are 56+8=64 bytes apart), so the
two-producer/consumer false-sharing concern is genuinely mitigated for
the *inter-field* case. The padding *before* `head` is missing, so
`head` may share a line with adjacent heap data, but in practice that
adjacent data is dead struct prologue, so it doesn't hurt.

Worth fixing if you actually care: bump `_pad0` to `[64]byte` *before*
`head`, or use `runtime/internal/sys.CacheLinePadSize` if you don't
mind importing an internal package.

Otherwise the ring is correct — `-race` finds nothing.

### 5. `buffer.go` is broken sync.Cond — and dead code

```go
func (b *Buffer) StoreTo(f func([]byte)) {
    b.c.L.Lock()
    b.c.Wait()        // ← no predicate loop, no flag
    f(b.buf[:])
    b.c.L.Unlock()
}
```

This is a textbook misuse of `sync.Cond`:

- `Wait` is not in a `for !predicate { ... }` loop — a spurious wakeup
  or a `Signal` that arrives before `Wait` is called will desync the
  state.
- There is no `ready` flag at all. The first `Signal` from `LoadFrom`
  arriving before any `StoreTo` has called `Wait` is **lost**, and the
  next `StoreTo` will block forever.
- `LoadFrom` calls `Signal` after `Unlock`, with no flag set, so even
  if `StoreTo` is waiting, it has no way to know whether new data is
  actually present.

Nothing in the benchmark or test suite uses `buffer.go`. It's dead
code, but it's currently the most broken file in the package.

### 6. `chan.go` allocates 64 MiB/Start; `PoolChan` doesn't actually fix the alloc count

The Chan producer does `make([]byte, S64k)` per task — 1000 × 64KiB =
62.5 MiB allocated per call. The pool variant looks like it should
recycle, and it does — but its `allocs/op` is **identical** at 1009:

```go
for range cnt {
    buf := p.P.Get().(*bytes.Buffer)
    if _, err := io.CopyN(buf, f, S64k); err != nil {
        return
    }
    ch <- buf
}
```

`io.CopyN(dst, src, n)` is `io.Copy(dst, io.LimitReader(src, n))`, and
`io.LimitReader` does `return &LimitedReader{src, n}` — one heap
allocation per call. With cnt=1000 that's exactly 1000 LimitedReader
escapes, matching the observed 1009 allocs/op. The pool successfully
saves the 64KiB×1000 of buffer allocation (`B/op` drops from 62.5 MiB
to 70 KiB), but the **alloc count** is dominated by io.CopyN's helper
struct, not the buffer. A direct `io.ReadFull(f, buf.Bytes()[:cap(buf.Bytes())])`
(after `buf.Reset(); buf.Grow(S64k)`) would drop allocs to single
digits.

### 7. Tests don't return pooled buffers

```go
func TestPoolProducer(t *testing.T) {
    p := sync.NewPoolProd()
    ch := p.Start(10)
    for b := range ch {
        buf := b.Bytes()
        ...
    }
    // buffers never Put back
}
```

The pool is GC-collected at process exit so it's not a leak, but the
test models the wrong usage pattern — if anyone copies this, they'll
defeat the pool entirely.

### Non-bugs

- The loop-variable closures (`for i := range RunTimes { wg.Go(func() { sink[i] = ... }) }`)
  are correct under Go 1.22+ per-iteration variables; project is 1.25.
- `Signal` is called outside the lock in several places — this is allowed
  by `sync.Cond` (the `notifyList` ticket is registered under the lock
  inside `Wait`, so no missed wakeup).
- Ring's `done` writes and reads are all atomic, no race.

---

## Results — explained

`go test -bench=. -benchmem -benchtime=3s -count=3` aggregated with
`benchstat`:

| Bench     | sec/op    | B/op       | allocs/op | per-task allocs                    |
|-----------|----------:|-----------:|----------:|:-----------------------------------|
| `Seq`     | 296.5 ms  | 120 B      |        3  | 0 (buf is in the struct)           |
| `Chan`    | 172.7 ms  | 62.5 MiB   |    1,009  | 1 — `make([]byte, S64k)` per chunk |
| `PoolChan`| 166.1 ms  | 69.77 KiB  |    1,009  | 1 — `io.LimitReader` inside CopyN  |
| `Cond`    | 161.8 ms  | 441 B      |        8  | 0 (bufs in struct)                 |
| `Ring`    | 164.2 ms  | 152 B      |        4  | 0 (slots in struct)                |

### Why Seq is 1.8× slower than the rest

The work splits cleanly into two stages:

| Stage           | Rate (this machine) | 64 MiB takes |
|-----------------|---------------------|--------------|
| /dev/urandom    | ~400 MB/s           | ~160 ms      |
| SHA-256         | ~460 MB/s           | ~140 ms      |

- Seq runs them in series: 160 + 140 = ~300 ms. ✓ (296 ms observed)
- The four parallel variants overlap them: `max(160, 140) = ~160 ms` ✓
  (162–173 ms observed)

### Why the four parallel variants are all bunched at ~165 ms

The bottleneck is **/dev/urandom**, not the synchronisation primitive.
Whether the producer hands buffers off through a channel, a Cond
ping-pong, or a ring, it spends most of its life sleeping in `read(2)`.
The differences in synchronisation overhead would have to be > ~10 ms /
1000 tasks = 10 µs/task to show through the noise — and the actual
overhead per task is more like 100 ns–1 µs.

**Implication: this benchmark does not actually measure what its name
suggests.** It measures urandom throughput, with sync primitives as a
sidecar. To make it a sync benchmark, replace `/dev/urandom` with a
fast deterministic source (e.g. `rand.Read` from `math/rand/v2`, or a
ChaCha20 stream) so the producer is CPU-bound at hundreds of MB/s and
the synchronisation cost can dominate.

### Why allocation numbers spread so widely

- **Seq (3 allocs)** — file handle + a couple of internal `os` bits.
  The 64 KiB buffer lives in the struct (`buf [S64k]byte`), so no per-
  chunk alloc.
- **Chan (1009 allocs, 62.5 MiB)** — `make([]byte, S64k)` per chunk:
  1000 × 64 KiB. The +9 is goroutine + channel + file scaffolding.
- **PoolChan (1009 allocs, 70 KiB)** — pool recycles the buffers
  successfully (B/op drops 900×), but `io.CopyN`'s internal
  `LimitedReader` allocates ~24 B per call. 1000 × 24 ≈ 24 KiB of the
  observed 70 KiB.
- **Cond (8 allocs)** — `buf1`/`buf2` are arrays inside the struct;
  the 8 allocs are 3 goroutines + 2 Conds + 1 resCh + 2 file ops.
- **Ring (4 allocs)** — slots are inside the struct; 4 allocs are
  producer goroutine + 2 Conds + file open.

### Why `PoolChan` does *not* beat `Chan` despite the 900× drop in bytes/op

Both are urandom-bound. The 62.5 MiB allocator pressure in `Chan` is
distributed over ~160 ms in a single producer goroutine, and Go's
bump allocator is very fast at sequential single-threaded 64 KiB
allocations. The GC does run, but most of its work overlaps with the
producer's urandom blocking. To see PoolChan win, you'd need a faster
source so allocation pressure actually competes for CPU.

### Why `Cond` and `Ring` don't beat `Chan`

Same reason — the urandom floor is shared. The fancier primitives
matter when synchronisation overhead is on the critical path; here it
isn't.

---

## Suggested next steps if you want to fix it

1. **Fix the cond.go data races.** Make the producer wait on a
   `ready1 == false` predicate before writing `buf1` (real ping-pong),
   and keep `done` consistently under one cond's lock or use
   `atomic.Bool` consistently. Right now the benchmark may be producing
   garbage hash sums; nothing checks because the result is discarded.

2. **Replace `/dev/urandom` with a CPU-bound stream.** As the bench
   stands, all four parallel variants are pinned to urandom's
   ~160 ms floor. With a faster source (e.g. ChaCha20 PRNG), the
   ranking between Chan/PoolChan/Cond/Ring would actually reflect
   synchronisation cost.

3. **Remove or fix `buffer.go`.** It's broken (no predicate loop, no
   flag) and unused. Either delete it or make it a working buffered
   handoff.

4. **Run with `-count >= 6`.** Current numbers report `± ∞`
   confidence — benchstat needs at least 6 samples. Three runs aren't
   statistically meaningful.

5. **Fix the misleading alloc accounting in PoolChan.** Swap
   `io.CopyN(buf, f, S64k)` for `io.ReadFull(f, b)` where `b` is the
   buffer's underlying slice — the LimitedReader escape disappears and
   allocs/op drops from 1009 to single digits, properly showcasing
   what the pool is for.

6. **Add at least one unit test for `Ring` and `Cond`.** A test that
   computes a known checksum (replace urandom with a deterministic
   source) would have caught both the `p.done` race (under `-race`)
   and the buf1 race (silently wrong output).
