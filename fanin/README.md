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
BenchmarkWorkerPoolCanonical-16                    39284             30863 ns/op            1030 B/op         16 allocs/op
BenchmarkWorkerPoolReflect-16                       3283            338913 ns/op          269879 B/op       4016 allocs/op
BenchmarkWorkerPoolLoop-16                         18078             66086 ns/op            1024 B/op         16 allocs/op
BenchmarkWorkerPoolBatch4-16                       22622             52930 ns/op            1027 B/op         16 allocs/op
BenchmarkWorkerPoolBatch2-16                       29120             41008 ns/op            1025 B/op         16 allocs/op
BenchmarkMetricsCanonical-16                        4431            267459 ns/op            6502 B/op        100 allocs/op
BenchmarkMetricsReflect-16                           124           9595376 ns/op         7369241 B/op     104129 allocs/op
BenchmarkMetricsLoop-16                             5350            208171 ns/op            6464 B/op        100 allocs/op
BenchmarkMetricsBatch4-16                           3693            310419 ns/op            6416 B/op        100 allocs/op
BenchmarkMetricsBatch2-16                           3896            300377 ns/op            6422 B/op        100 allocs/op
BenchmarkHugeSourceCountCanonical-16                  27          42910864 ns/op           88452 B/op       1086 allocs/op
BenchmarkHugeSourceCountReflect-16                     1        12611724803 ns/op       6944206800 B/op 100402478 allocs/op
BenchmarkHugeSourceCountBatch4-16                     37          27370017 ns/op         1028057 B/op      14562 allocs/op
BenchmarkHugeSourceCountBatch2-16                     32          38751360 ns/op           66124 B/op       1022 allocs/op
BenchmarkHugeSourceCountLoop-16                       69          17088617 ns/op           65779 B/op       1018 allocs/op
```
