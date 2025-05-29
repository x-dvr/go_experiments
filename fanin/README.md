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
BenchmarkWorkerPoolCanonical-16                    33226             35925 ns/op             780 B/op         16 allocs/op
BenchmarkWorkerPoolReflect-16                       4495            246157 ns/op          174897 B/op       3216 allocs/op
BenchmarkWorkerPoolLoop-16                         12849             93811 ns/op             770 B/op         16 allocs/op
BenchmarkWorkerPoolBatch4-16                       24518             48428 ns/op             769 B/op         16 allocs/op
BenchmarkWorkerPoolBatch2-16                       26648             44263 ns/op             769 B/op         16 allocs/op
BenchmarkMetricsCanonical-16                        4347            265471 ns/op            4891 B/op        100 allocs/op
BenchmarkMetricsReflect-16                           124           9567865 ns/op         7367975 B/op     104133 allocs/op
BenchmarkMetricsLoop-16                             5588            205936 ns/op            4885 B/op        101 allocs/op
BenchmarkMetricsBatch4-16                           3796            307656 ns/op            4828 B/op        100 allocs/op
BenchmarkMetricsBatch2-16                           3993            298389 ns/op            4813 B/op        100 allocs/op
BenchmarkHugeSourceCountCanonical-16                 300           3988277 ns/op           50906 B/op       1015 allocs/op
BenchmarkHugeSourceCountReflect-16                     1        1254595024 ns/op        694566576 B/op  10042435 allocs/op
BenchmarkHugeSourceCountBatch4-16                    433           2691683 ns/op          130797 B/op       2163 allocs/op
BenchmarkHugeSourceCountBatch2-16                    351           3441153 ns/op           48658 B/op       1006 allocs/op
BenchmarkHugeSourceCountLoop-16                      825           1447144 ns/op           48899 B/op       1009 allocs/op
```
