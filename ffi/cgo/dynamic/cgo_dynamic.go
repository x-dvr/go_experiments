// Package dynamic exposes the bench_* functions from libffilib loaded as a
// shared library at runtime (resolved via -rpath at link time).
package dynamic

/*
#cgo CFLAGS: -I${SRCDIR}/../../c
#cgo LDFLAGS: -L${SRCDIR}/../../lib -lffilib -Wl,-rpath,${SRCDIR}/../../lib

#include <stdlib.h>
#include "ffilib.h"

extern int32_t goIntCallbackTrampoline(int32_t);
extern int64_t goStructCallbackTrampoline(BenchPoint);

static int32_t call_int_cb(int32_t n, int32_t iters) {
    return bench_call_int_callback(goIntCallbackTrampoline, n, iters);
}

static int64_t call_struct_cb(BenchPoint p) {
    return bench_call_struct_callback(goStructCallbackTrampoline, p);
}
*/
import "C"

import (
	"unsafe"
)

type Point struct {
	X, Y, Z, W int32
}

func AddInts(a, b int32) int32 {
	return int32(C.bench_add_ints(C.int32_t(a), C.int32_t(b)))
}

func Strlen(s string) int {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return int(C.bench_strlen(cs))
}

// StrlenNoCopy avoids the C.CString allocation by reusing the Go string's
// backing bytes. Caller must guarantee the string is NUL-terminated; we
// append a trailing zero byte here so the C side sees a valid C string.
func StrlenNoCopy(s string) int {
	buf := append([]byte(s), 0)
	return int(C.bench_strlen((*C.char)(unsafe.Pointer(&buf[0]))))
}

func SumBytes(data []byte) int64 {
	if len(data) == 0 {
		return int64(C.bench_sum_bytes(nil, 0))
	}
	return int64(C.bench_sum_bytes(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
	))
}

func PointAdd(a, b Point) Point {
	ca := C.BenchPoint{x: C.int32_t(a.X), y: C.int32_t(a.Y), z: C.int32_t(a.Z), w: C.int32_t(a.W)}
	cb := C.BenchPoint{x: C.int32_t(b.X), y: C.int32_t(b.Y), z: C.int32_t(b.Z), w: C.int32_t(b.W)}
	cr := C.bench_point_add(ca, cb)
	return Point{X: int32(cr.x), Y: int32(cr.y), Z: int32(cr.z), W: int32(cr.w)}
}

func CallIntCallback(n, iters int32) int32 {
	return int32(C.call_int_cb(C.int32_t(n), C.int32_t(iters)))
}

func CallStructCallback(p Point) int64 {
	cp := C.BenchPoint{x: C.int32_t(p.X), y: C.int32_t(p.Y), z: C.int32_t(p.Z), w: C.int32_t(p.W)}
	return int64(C.call_struct_cb(cp))
}

//export goIntCallbackTrampoline
func goIntCallbackTrampoline(n C.int32_t) C.int32_t {
	return n + 1
}

//export goStructCallbackTrampoline
func goStructCallbackTrampoline(p C.BenchPoint) C.int64_t {
	return C.int64_t(int64(p.x) + int64(p.y) + int64(p.z) + int64(p.w))
}
