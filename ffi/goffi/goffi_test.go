package goffibench

import (
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

func TestAddInts(t *testing.T) {
	if got := AddInts(2, 3); got != 5 {
		t.Fatalf("AddInts(2,3) = %d, want 5", got)
	}
}

func TestStrlen(t *testing.T) {
	if got := Strlen("hello"); got != 5 {
		t.Fatalf("Strlen(\"hello\") = %d, want 5", got)
	}
}

func TestSumBytes(t *testing.T) {
	if got := SumBytes([]byte{1, 2, 3, 4, 5}); got != 15 {
		t.Fatalf("SumBytes = %d, want 15", got)
	}
}

func TestPointAdd(t *testing.T) {
	r := PointAdd(Point{1, 2, 3, 4}, Point{10, 20, 30, 40})
	want := Point{11, 22, 33, 44}
	if r != want {
		t.Fatalf("PointAdd = %v, want %v", r, want)
	}
}

func TestIntCallback(t *testing.T) {
	if got := CallIntCallback(0, 5); got != 5 {
		t.Fatalf("CallIntCallback(0, 5) = %d, want 5", got)
	}
}

func TestStructCallback(t *testing.T) {
	if got := CallStructCallback(Point{1, 2, 3, 4}); got != 10 {
		t.Fatalf("CallStructCallback = %d, want 10", got)
	}
}

var (
	benchStr   = "Hello, FFI benchmark!\x00"
	benchBytes = []byte("The quick brown fox jumps over the lazy dog.")
)

func BenchmarkAddInts(b *testing.B) {
	var r int32
	for i := 0; i < b.N; i++ {
		r = AddInts(int32(i), 7)
	}
	_ = r
}

func BenchmarkStrlen(b *testing.B) {
	var r int
	for i := 0; i < b.N; i++ {
		r = Strlen(benchStr)
	}
	_ = r
}

func BenchmarkSumBytes(b *testing.B) {
	var r int64
	for i := 0; i < b.N; i++ {
		r = SumBytes(benchBytes)
	}
	_ = r
}

func BenchmarkPointAdd(b *testing.B) {
	p1 := Point{1, 2, 3, 4}
	p2 := Point{5, 6, 7, 8}
	var r Point
	for i := 0; i < b.N; i++ {
		r = PointAdd(p1, p2)
	}
	_ = r
}

func BenchmarkIntCallback(b *testing.B) {
	var r int32
	for i := 0; i < b.N; i++ {
		r = CallIntCallback(0, 1)
	}
	_ = r
}

func BenchmarkStructCallback(b *testing.B) {
	p := Point{1, 2, 3, 4}
	var r int64
	for i := 0; i < b.N; i++ {
		r = CallStructCallback(p)
	}
	_ = r
}

// ---------------------------------------------------------------------------
// "Raw" idiomatic goffi benchmarks: argv slice and value storage are hoisted
// out of the loop. This mirrors the prepare-once / call-many style goffi's
// own benchmarks use, which is the basis for the 88-114 ns/op figure cited
// in its README. Per-call escapes are avoided by reusing the same memory.
// ---------------------------------------------------------------------------

func BenchmarkAddIntsRaw(b *testing.B) {
	var a, bb, r int32
	bb = 7
	args := []unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&bb)}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a = int32(i)
		_ = ffi.CallFunction(&addIntsCIF, addIntsSym, rPtr, args)
	}
}

func BenchmarkStrlenRaw(b *testing.B) {
	buf := append([]byte(benchStr), 0)
	strPtr := unsafe.Pointer(&buf[0])
	var r uint64
	args := []unsafe.Pointer{unsafe.Pointer(&strPtr)}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffi.CallFunction(&strlenCIF, strlenSym, rPtr, args)
	}
}

func BenchmarkSumBytesRaw(b *testing.B) {
	dataPtr := unsafe.Pointer(&benchBytes[0])
	length := uint64(len(benchBytes))
	var r int64
	args := []unsafe.Pointer{unsafe.Pointer(&dataPtr), unsafe.Pointer(&length)}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffi.CallFunction(&sumBytesCIF, sumBytesSym, rPtr, args)
	}
}

func BenchmarkPointAddRaw(b *testing.B) {
	p1 := Point{1, 2, 3, 4}
	p2 := Point{5, 6, 7, 8}
	var r Point
	args := []unsafe.Pointer{unsafe.Pointer(&p1), unsafe.Pointer(&p2)}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffi.CallFunction(&pointAddCIF, pointAddSym, rPtr, args)
	}
}

func BenchmarkIntCallbackRaw(b *testing.B) {
	cb := intCallbackPtr
	var n, iters, r int32
	iters = 1
	args := []unsafe.Pointer{
		unsafe.Pointer(&cb),
		unsafe.Pointer(&n),
		unsafe.Pointer(&iters),
	}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffi.CallFunction(&callIntCbCIF, callIntCbSym, rPtr, args)
	}
}

func BenchmarkStructCallbackRaw(b *testing.B) {
	cb := structCallbackPtr
	p := Point{1, 2, 3, 4}
	var r int64
	args := []unsafe.Pointer{unsafe.Pointer(&cb), unsafe.Pointer(&p)}
	rPtr := unsafe.Pointer(&r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffi.CallFunction(&callStructCbCIF, callStructCbSym, rPtr, args)
	}
}
