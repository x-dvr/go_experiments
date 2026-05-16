# Experiment with different worker pool implementations

Each pool runs the same workload: 10,000 tasks, each iterating Fibonacci to
1e5 (~200 µs CPU each) and writing the result into a shared sink slice.

## Pool flavors

| Pool             | Wiring                                                                |
|------------------|-----------------------------------------------------------------------|
| `NoPool`         | One `wg.Go(task)` per task — `RunTimes` goroutines per batch          |
| `ErrGroup`       | `errgroup.Group.SetLimit(PoolCap)`, spawns goroutines up to the cap   |
| `AntsPool`       | `github.com/panjf2000/ants/v2` pre-allocated pool                     |
| `SemaphorePool`  | `chan struct{}` semaphore + `wg.Go(task)` per task                    |
| `PreallocPool`   | `PoolCap` workers + one shared `chan Task` buffer=`PoolCap`           |
| `StaticPool`     | Like Prealloc but task is `tsk{arg, iter}` value (zero per-task allocs) |
| `RoundRobinPool` | `PoolCap` workers + per-worker channel buffer=`PoolCap`               |

All pools are constructed **once** outside `b.Loop()` and reused across
iterations via a per-batch `Drain()` (workers stay alive between batches).
This isolates the per-task submit-and-drain cost from the one-time
construction cost — the latter is amortised over `b.N`.

`PoolCap` defaults to `runtime.NumCPU() * 2` (= 32 on the bench machine).

## Run benchmarks

```sh
# run all benchmarks
go test -bench=. -benchmem -benchtime=10s ./worker_pool

# stable numbers — for benchstat
go test -bench=. -benchmem -benchtime=10s -count=6 ./worker_pool > bench.txt
benchstat bench.txt

# profile one benchmark
go test ./worker_pool -bench=^BenchmarkNoPool$ -benchmem \
    -memprofile memprofile.out -cpuprofile profile.out
go tool pprof -web memprofile.out

# trace one benchmark
go test ./worker_pool -bench=^BenchmarkNoPool$ -trace=trace.out
go tool trace trace.out
```

Measure CPU usage:
```sh
perf stat go test -bench=^BenchmarkPreallocPool$ -benchtime=20s ./worker_pool
perf stat go test -bench=^BenchmarkNoPool$ -benchtime=20s ./worker_pool
```

- Reduce memory allocations, so Go spends less CPU time and memory bandwidth on garbage collection
- Analyze heap profile with `go tool pprof -alloc_objects` — it shows the main sources of memory allocations
- Use `sync.Pool` for CPU-bound code
- Reduce the number of live objects with pointers in heap, so Go GC spends less CPU time and memory bandwidth on visiting pointers
- Analyze heap profile with `go tool pprof -inuse_objects` — it shows the main sources of live objects

## Results

`go test -bench=. -benchtime=10s -count=6`, aggregated with `benchstat`,
Intel i7-10870H @ 2.20 GHz, Linux amd64, Go 1.25:

| Bench              | sec/op           | B/op            | allocs/op |
|--------------------|------------------|-----------------|-----------|
| `RoundRobinPool`   |  61.01 ms ± 2%   |       3.5 B ±43% |    0      |
| `NoPool`           |  70.68 ms ± 7%   |   390.6 KiB ± 8% | 20,000    |
| `AntsPool`         |  73.93 ms ± 1%   |   234.7 KiB ± 0% | 10,000    |
| `ErrGroup`         |  74.95 ms ± 2%   |   390.6 KiB ± 0% | 20,000    |
| `PreallocPool`     |  77.27 ms ± 0%   |   156.3 KiB ± 0% | 10,000    |
| `StaticPool`       |  79.14 ms ± 1%   |       5.0 B ±320%|    0      |
| `SemaphorePool`    |  79.59 ms ± 1%   |   625.0 KiB ± 0% | 30,000    |

(The 3-5 B/op figures with high noise on the zero-alloc pools are GC
overhead the bench framework attributes to the iteration; effectively zero.)

See [analysis.md](analysis.md) for the full discussion of why
RoundRobinPool now wins and where each pool's per-task allocations come
from.
