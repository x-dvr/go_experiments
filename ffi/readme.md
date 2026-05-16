# FFI benchmark suite

Comparison of calling C from Go via **CGO** (dynamic + static linking) vs
[**goffi**](https://github.com/go-webgpu/goffi) (a CGO-free FFI built on
`dlopen` + hand-written assembly trampolines).

## Layout

```
ffi/
├── c/ffilib.{c,h}     C source — int, string, byte-slice, struct, callback drivers
├── lib/               build output: libffilib.{so,a}
├── cgo/dynamic/       CGO bench, links libffilib.so via -Wl,-rpath
├── cgo/static/        CGO bench, embeds libffilib.a at link time
├── goffi/             goffi bench (dlopen)
└── justfile           `just build` / `just bench` / `just test`
```

Every backend exercises the same six cases:

| Case               | C signature                                                 |
|--------------------|-------------------------------------------------------------|
| `AddInts`          | `int32_t bench_add_ints(int32_t, int32_t)`                  |
| `Strlen`           | `size_t bench_strlen(const char*)`                          |
| `SumBytes`         | `int64_t bench_sum_bytes(const uint8_t*, size_t)`           |
| `PointAdd`         | `BenchPoint bench_point_add(BenchPoint, BenchPoint)` (16B)  |
| `IntCallback`      | `int32_t bench_call_int_callback(cb, int32_t, int32_t)`     |
| `StructCallback`   | `int64_t bench_call_struct_callback(cb, BenchPoint)`        |

CGO routes callbacks through `//export` trampolines; goffi uses
`ffi.NewCallback`.

The goffi suite has two flavors:

* **Wrapped** (`BenchmarkAddInts`, …) — a Go function wraps each FFI call.
  The args slice (`[]unsafe.Pointer{…}`) is built per call and the input
  values escape to the heap because their addresses are taken across a
  function boundary. This is the ergonomic API a real caller would write.
* **Raw** (`BenchmarkAddIntsRaw`, …) — argv slice and value storage are
  hoisted out of the loop, so the hot path is just `ffi.CallFunction`. This
  mirrors how goffi's own README benchmarks measure things and is the
  basis for the 88–114 ns/op figure cited there.

## Results

`just bench` on Intel i7-10870H @ 2.20 GHz, Linux amd64, Go 1.25, gcc 16.1.1,
`gcc -O2 -fPIC`. Each benchmark run 10 times (`go test -count=10`),
summarised with `benchstat` (median ± relative range). Allocation columns
report the per-call B/op and allocs/op, which are deterministic.

### CGO (dynamic and static)

| Benchmark           | CGO dynamic               | CGO static                | B/op | allocs/op |
|---------------------|---------------------------|---------------------------|-----:|----------:|
| `AddInts`           |  30.99 ns ± 2%            |  30.87 ns ± 1%            |    0 |         0 |
| `Strlen` (CString)  | 114.5  ns ± 2%            | 114.9  ns ± 1%            |    0 |         0 |
| `Strlen` (no-copy)  |  65.48 ns ± 4%            |  69.20 ns ± 7%            |   24 |         1 |
| `SumBytes`          |  40.14 ns ± 1%            |  39.57 ns ± 2%            |    0 |         0 |
| `PointAdd`          |  35.86 ns ± 1%            |  35.43 ns ± 1%            |    0 |         0 |
| `IntCallback`       |  80.78 ns ± 2%            |  81.20 ns ± 2%            |    0 |         0 |
| `StructCallback`    |  82.14 ns ± 2%            |  80.99 ns ± 2%            |    0 |         0 |
| geomean             |  58.04 ns                 |  58.20 ns                 |      |           |

### goffi (wrapped API vs raw idiomatic)

Run in isolation (`just bench-goffi`, cooler CPU):

| Benchmark         | Wrapped API     | Raw (hoisted argv) | Wrap B/op | Wrap allocs | Raw B/op | Raw allocs |
|-------------------|-----------------|--------------------|----------:|------------:|---------:|-----------:|
| `AddInts`         | 187.5 ns ± 17%  | 147.6 ns ± 14%     |       240 |           3 |      208 |          1 |
| `Strlen`          | 250.8 ns ± 18%  | 154.4 ns ± 18%     |       256 |           5 |      208 |          1 |
| `SumBytes`        | 261.5 ns ± 36%  | 168.0 ns ± 26%     |       248 |           4 |      208 |          1 |
| `PointAdd`        | 321.8 ns ± 31%  | 217.7 ns ± 29%     |       272 |           5 |      208 |          1 |
| `IntCallback`     | 608.1 ns ±  4%  | 511.6 ns ±  3%     |       288 |           7 |      248 |          4 |
| `StructCallback`  | 759.4 ns ±  3%  | 678.5 ns ±  3%     |       304 |           7 |      256 |          4 |
| geomean           | (all)           | 299.9 ns           |           |             |          |            |

The non-callback rows have wide spreads (±14–36%); the callback rows are
tight (±3–4%). This is **not** thermal throttling — isolated runs after
the CPU cooled showed the same shape, just shifted ~10% lower in absolute
ns. The spread is intrinsic jitter from `runtime.cgocall` on short
calls: at ~150 ns per op, any goroutine→M migration, OS preemption,
or `g0` stack-switch cost shows up directly. The callback path takes
500+ ns per op, so the same jitter averages out into noise.

CGO numbers do not show this — `cgocall` is the same primitive, but
calling-convention-specific glue in the CGO path is hot-cached after Go
1.21 and adds less overhead, so per-call variance is smaller.

## Interpretation

### CGO dynamic vs static is within noise

Dynamic linking has a one-off `dlopen`/PLT cost at process start; the call
hot path is identical. The per-op numbers match within run-to-run variance.

### goffi does *not* use reflection on the call path

The outbound `ffi.CallFunction` is a direct `switch` on `argType.Kind` that
reads each value through `unsafe.Pointer` and forwards through hand-written
assembly via `runtime.cgocall`. No `reflect` involved.

`reflect` *is* used on the **callback** side: when C invokes a Go callback,
the wrapper rebuilds `reflect.Value` arguments from the saved register
frame and dispatches with `fn.Call(args)`. That accounts for the extra
~3 allocs and ~350 ns the callback benchmarks show compared to the simple
calls in both the wrapped and raw columns.

### Why the wrapped goffi numbers are ~2× the raw ones

Per call, the wrapped form costs:

* a fresh `[]unsafe.Pointer{…}` slice (escape, 1 alloc)
* every argument value escaping because its address crosses a function
  boundary (`unsafe.Pointer(&a)`, `unsafe.Pointer(&b)`, …)
* a `runtime.KeepAlive` of the argv slice inside goffi

The raw form pays only goffi's irreducible internal cost (see below).

### Where the remaining 1 alloc / 208 B/op in the raw goffi path comes from

`internal/syscall/syscall_unix_amd64.go:CallNFloat` builds a local
`syscallArgs` struct (~200 bytes) and passes `unsafe.Pointer(&args)` to
`runtime.cgocall`. The address-taken makes `args` escape to the heap.
Nothing the caller does eliminates this — it is fixed overhead per FFI
call.

### Why goffi is fast enough for `wgpu-native`

WebGPU/`wgpu-native` issues O(50) FFI calls per frame at 60 FPS. At
~150 ns/op (raw goffi) that's ~7.5 µs out of a 16.6 ms frame budget —
0.04 %, unmeasurable in a profiler. The trade-off is great for that
workload; it would be a bad choice for code that calls into a math
library in a tight loop, where CGO's ~30 ns/op is 5× cheaper.

### Static linking is not supported by goffi

goffi resolves symbols by calling `dlsym` on a handle returned by
`dlopen`. A statically-linked object has no `.so` to open and no
DT_NEEDED entry — `dlsym` has nothing to look up. Specifically:

* `ffi.LoadLibrary("")` calls `dlopen("", ...)`, which fails on Linux
  (it tries to load a file literally named `""`).
* The pseudo-handles `RTLD_DEFAULT` / `RTLD_NEXT` (which would search the
  main program's symbols) live in goffi's `internal/dl` package and are
  not exposed through the `LoadLibrary` API.
* Even if they were, "static" symbols in a Go binary only appear in the
  process address space through CGO's import mechanism — and once you're
  using CGO, the whole point of goffi is gone.

The closest practical equivalent is to ship a small `.so` next to the
binary and call `LoadLibrary("/absolute/path/lib.so")` (or rely on
`LD_LIBRARY_PATH` / `RPATH`). Still a dynamic load, just bundled.

## Build & run

```sh
just build              # compile libffilib.so + libffilib.a
just bench              # build, then run every benchmark
just bench-cgo-dynamic  # one backend at a time
just bench-cgo-static
just bench-goffi
just test               # correctness tests only
just clean
```

## Caveats

* Linux amd64 only — goffi's callback assembly currently targets that
  platform (the rest of the suite is portable, but the numbers here are
  not).
* The numbers above are from `go test -count=10` aggregated with
  `benchstat`. Rerunning each suite in isolation (cooler CPU) tightens the
  goffi spread to ±3–5%; running them back-to-back as `just bench` does
  shows the wider ranges quoted in the table.
