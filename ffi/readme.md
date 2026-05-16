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
`gcc -O2 -fPIC`. ns/op (lower = better), B/op, allocs/op.

### CGO (dynamic and static)

| Benchmark           | CGO dynamic                | CGO static                 |
|---------------------|----------------------------|----------------------------|
| `AddInts`           |   30.66 ns,   0 B, 0 allocs|   31.32 ns,   0 B, 0 allocs|
| `Strlen` (CString)  |  111.3  ns,   0 B, 0 allocs|  114.9  ns,   0 B, 0 allocs|
| `Strlen` (no-copy)  |   64.76 ns,  24 B, 1 alloc |   65.35 ns,  24 B, 1 alloc |
| `SumBytes`          |   39.43 ns,   0 B, 0 allocs|   38.88 ns,   0 B, 0 allocs|
| `PointAdd`          |   35.28 ns,   0 B, 0 allocs|   35.34 ns,   0 B, 0 allocs|
| `IntCallback`       |   80.53 ns,   0 B, 0 allocs|   79.96 ns,   0 B, 0 allocs|
| `StructCallback`    |   80.27 ns,   0 B, 0 allocs|   81.02 ns,   0 B, 0 allocs|

### goffi (wrapped API vs raw idiomatic)

| Benchmark         | Wrapped API                | Raw (hoisted argv/storage) |
|-------------------|----------------------------|----------------------------|
| `AddInts`         |  282.1 ns, 240 B, 3 allocs |  138.9 ns, 208 B, 1 alloc  |
| `Strlen`          |  317.2 ns, 256 B, 5 allocs |  122.8 ns, 208 B, 1 alloc  |
| `SumBytes`        |  296.1 ns, 248 B, 4 allocs |  188.4 ns, 208 B, 1 alloc  |
| `PointAdd`        |  312.2 ns, 272 B, 5 allocs |  181.2 ns, 208 B, 1 alloc  |
| `IntCallback`     |  573.7 ns, 288 B, 7 allocs |  507.3 ns, 248 B, 4 allocs |
| `StructCallback`  |  739.1 ns, 304 B, 7 allocs |  690.8 ns, 256 B, 4 allocs |

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

WebGPU/`wgpu-native` issues O(50) FFI calls per frame at 60 FPS. At ~150
ns/op that's ~7.5 µs out of a 16.6 ms frame budget — 0.04 %, unmeasurable
in a profiler. The trade-off is great for that workload; it would be a
bad choice for code that calls into a math library in a tight loop, where
CGO's ~30 ns/op is 5× cheaper.

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
* Benchmarks were run once; treat absolute numbers as ballpark and rerun
  with `-count=N` for serious comparisons.
