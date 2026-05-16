# Sync experiment

Five variants of a 1000 × 64 KiB urandom-read → SHA-256-sum pipeline,
exploring different producer/consumer hand-off styles.

## Variants

| Variant     | Producer / Consumer wiring                            |
|-------------|-------------------------------------------------------|
| `Seq`       | One goroutine: read then hash, in place               |
| `Chan`      | Producer goroutine → `chan []byte` size 1 → consumer  |
| `PoolChan`  | Same as Chan, buffers recycled via `sync.Pool`        |
| `Cond`      | Producer + two consumers, two `sync.Cond` ping-pong   |
| `Ring`      | Bounded SPSC ring (size 4) with head/tail atomics + 2 Cond |

## Run benchmarks

```sh
go test -bench=. -benchmem -benchtime=3s ./sync

# stable numbers — for benchstat
go test -bench=. -benchmem -benchtime=3s -count=6 ./sync > bench.txt
benchstat bench.txt

# verify no data races
go test -race -bench=. -benchtime=10x ./sync
```

## Results

`go test -bench=. -benchtime=3s -count=6` aggregated with `benchstat`,
Intel i7-10870H @ 2.20 GHz, Linux amd64, Go 1.25:

| Bench       | sec/op        | B/op            | allocs/op |
|-------------|---------------|-----------------|-----------|
| `Seq`       | 316.3 ms ± 3% |     120 B       |      3    |
| `Chan`      | 178.0 ms ± 2% |    62.50 MiB    |  1,010    |
| `PoolChan`  | 174.1 ms ± 2% |    18.44 KiB ±19% |    9    |
| `Cond`      | 173.7 ms ± 1% |     546.5 B ±34%|      8    |
| `Ring`      | 173.1 ms ± 2% |     152 B ±11%  |      4    |

All four parallel variants run race-clean under `-race`. The parallel
variants converge to ~173–178 ms because the bottleneck is
`/dev/urandom` read throughput (~400 MB/s on this kernel) — the
producer's read time exceeds the consumer's SHA-256 time, so the
synchronisation primitive doesn't matter much.

### Why `PoolChan` stores `*[]byte`, not `[]byte`

`sync.Pool.Put/Get` take/return `any`, which is a `(type, data)` pair
where `data` is one pointer-sized word. A slice header is 24 bytes
(`ptr` + `len` + `cap`), so storing a `[]byte` forces the runtime to
heap-allocate a copy of the slice header to back the `any` — one
alloc per `Put`, defeating the pool entirely (you avoid one 64 KiB
allocation but pay a 24 B slice-header allocation, and `allocs/op`
still grows linearly with task count). `*[]byte` fits directly in
the interface data word with no boxing.

Staticcheck flags the wrong pattern as
[SA6002](https://staticcheck.dev/docs/checks/#SA6002), and
`sync.Pool`'s own docs recommend pointer types for this reason.

See [analysis.md](analysis.md) for the full discussion of bugs found,
fixes applied, and result interpretation.
