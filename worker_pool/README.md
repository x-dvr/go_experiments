# Experiment with different worker pool implementations

More details [here](http://example.com)

Run benchmarks:
```sh
# run all benchmarks
go test -bench=. -benchmem -benchtime=10s ./worker_pool

# profile one benchmark
go test ./worker_pool -bench=^BenchmarkNoPool$ -benchmem -memprofile memprofile.out -cpuprofile profile.out
# show profile
go tool pprof -web memprofile.out

# trace one benchmark
go test ./worker_pool -bench=^BenchmarkNoPool$ -trace=trace.out
# show trace
go tool trace trace.out
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
BenchmarkNoPool-16                   277          43122852 ns/op         1648003 B/op     100099 allocs/op
BenchmarkErrGroup-16                 148          69116382 ns/op         4000014 B/op     200000 allocs/op
BenchmarkAntsPool-16                 183          64860110 ns/op         1602015 B/op     100020 allocs/op
BenchmarkSemaphorePool-16            152          78712478 ns/op         4000004 B/op     200000 allocs/op
BenchmarkPreallocPool-16             254          47502297 ns/op         1600002 B/op     100000 allocs/op
BenchmarkStaticPool-16               248          48050947 ns/op               2 B/op          0 allocs/op
BenchmarkRoundRobinPool-16           271          44089732 ns/op               2 B/op          0 allocs/op
PASS
ok      github.com/x-dvr/go_experiments/worker_pool     90.093s
```
