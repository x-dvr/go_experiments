# Experiment with different fan-in implementations

More details [here](http://example.com)

Run benchmarks:
```sh
go test -bench=. -benchmem
```

## Results

```
goos: linux
goarch: amd64
pkg: github.com/x-dvr/go_experiments/fanin
cpu: Intel(R) Core(TM) i7-10870H CPU @ 2.20GHz
BenchmarkMergeGoChanInt-16              67580131                17.68 ns/op            0 B/op          0 allocs/op
BenchmarkMergeReflectChanInt-16         59583638                19.28 ns/op            1 B/op          0 allocs/op
BenchmarkMergeReflectIterInt-16         15597328                69.38 ns/op           44 B/op          3 allocs/op
```
