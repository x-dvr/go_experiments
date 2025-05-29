# Experiment with different fan-in implementations

More details [here](http://example.com)

Run benchmarks:
```sh
# run all benchmarks
go test -bench=. -benchmem ./fanin

# profile one benchmark
go test -bench=^BenchmarkMetricsReflect$ -benchmem ./fanin -memprofile memprofile.out -cpuprofile profile.out
# show profile
go tool pprof -web memprofile.out
```

## Results

```
goos: linux
goarch: amd64
pkg: github.com/x-dvr/go_experiments/fanin
cpu: Intel(R) Core(TM) i7-10870H CPU @ 2.20GHz
BenchmarkMetricsCanonical-16               23522             51153 ns/op            6418 B/op        100 allocs/op
BenchmarkMetricsReflect-16                  1198            967134 ns/op          744086 B/op      10517 allocs/op
BenchmarkMetricsLoop-16                    14046             86055 ns/op            6400 B/op        100 allocs/op
BenchmarkWorkerPoolCanonical-16            38960             31365 ns/op            1024 B/op         16 allocs/op
BenchmarkWorkerPoolReflect-16               3234            342429 ns/op          269864 B/op       4016 allocs/op
BenchmarkWorkerPoolLoop-16                 18076             66331 ns/op            1024 B/op         16 allocs/op
```
