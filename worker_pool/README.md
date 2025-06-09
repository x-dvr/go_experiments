# Experiment with different fan-in implementations

More details [here](http://example.com)

Run benchmarks:
```sh
# run all benchmarks
go test -bench=. -benchmem -benchtime=10s ./worker_pool

# profile one benchmark
go test ./worker_pool -bench=^BenchmarkPreallocPool$ -benchmem -memprofile memprofile.out -cpuprofile profile.out
# show profile
go tool pprof -web memprofile.out
```

Measure CPU usage:
```sh
perf stat go test -bench=^BenchmarkPreallocPool$ -benchtime=20s ./worker_pool
perf stat go test -bench=^BenchmarkNoPool$ -benchtime=20s ./worker_pool

```

## Results

```
goos: linux
goarch: amd64
pkg: github.com/x-dvr/go_experiments/worker_pool
cpu: Intel(R) Core(TM) i7-10870H CPU @ 2.20GHz
BenchmarkNoPool-16                   328          36454247 ns/op         1619652 B/op     100041 allocs/op
BenchmarkErrGroup-16                 166          80118816 ns/op         4000030 B/op     200000 allocs/op
BenchmarkAntsPool-16                 202          58806556 ns/op         1603005 B/op     100031 allocs/op
BenchmarkSemaphorePool-16            152          78665797 ns/op         4000003 B/op     200000 allocs/op
BenchmarkPreallocPool-16             306          39126920 ns/op         1600001 B/op     100000 allocs/op
BenchmarkStaticPool-16               304          39326183 ns/op               1 B/op          0 allocs/op
BenchmarkRoundRobinPool-16           348          34369560 ns/op               1 B/op          0 allocs/op
```
