// Package goffibench drives the bench_* functions through the goffi
// (github.com/go-webgpu/goffi) dynamic-loading FFI. goffi only supports
// dlopen-style libraries, so there is no static-link variant for it.
package goffibench

import (
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

type Point struct {
	X, Y, Z, W int32
}

// pointTypeDescriptor: 16-byte {int32, int32, int32, int32} — two INTEGER
// eightbytes under the System V AMD64 ABI.
var pointTypeDescriptor = &types.TypeDescriptor{
	Kind:      types.StructType,
	Size:      16,
	Alignment: 4,
	Members: []*types.TypeDescriptor{
		types.SInt32TypeDescriptor,
		types.SInt32TypeDescriptor,
		types.SInt32TypeDescriptor,
		types.SInt32TypeDescriptor,
	},
}

var (
	libHandle unsafe.Pointer

	addIntsSym, strlenSym, sumBytesSym, pointAddSym unsafe.Pointer
	callIntCbSym, callStructCbSym                   unsafe.Pointer
	addIntsCIF, strlenCIF, sumBytesCIF, pointAddCIF types.CallInterface
	callIntCbCIF, callStructCbCIF                   types.CallInterface
	intCallbackPtr, structCallbackPtr               uintptr
)

func libraryPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "libffilib.so"
	}
	return filepath.Join(filepath.Dir(file), "..", "lib", "libffilib.so")
}

func mustPrepare(cif *types.CallInterface, ret *types.TypeDescriptor, args []*types.TypeDescriptor) {
	if err := ffi.PrepareCallInterface(cif, types.DefaultCall, ret, args); err != nil {
		panic(err)
	}
}

func mustSym(name string) unsafe.Pointer {
	sym, err := ffi.GetSymbol(libHandle, name)
	if err != nil {
		panic(fmt.Errorf("goffi: GetSymbol(%s): %w", name, err))
	}
	return sym
}

func init() {
	h, err := ffi.LoadLibrary(libraryPath())
	if err != nil {
		panic(fmt.Errorf("goffi: LoadLibrary(%s): %w", libraryPath(), err))
	}
	libHandle = h

	addIntsSym = mustSym("bench_add_ints")
	strlenSym = mustSym("bench_strlen")
	sumBytesSym = mustSym("bench_sum_bytes")
	pointAddSym = mustSym("bench_point_add")
	callIntCbSym = mustSym("bench_call_int_callback")
	callStructCbSym = mustSym("bench_call_struct_callback")

	mustPrepare(&addIntsCIF, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor, types.SInt32TypeDescriptor})

	mustPrepare(&strlenCIF, types.UInt64TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor})

	mustPrepare(&sumBytesCIF, types.SInt64TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor, types.UInt64TypeDescriptor})

	mustPrepare(&pointAddCIF, pointTypeDescriptor,
		[]*types.TypeDescriptor{pointTypeDescriptor, pointTypeDescriptor})

	mustPrepare(&callIntCbCIF, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor,
			types.SInt32TypeDescriptor,
			types.SInt32TypeDescriptor,
		})

	mustPrepare(&callStructCbCIF, types.SInt64TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor, pointTypeDescriptor})

	intCallbackPtr = ffi.NewCallback(intCallback)
	structCallbackPtr = ffi.NewCallback(structCallback)
}

func intCallback(n int32) int32 {
	return n + 1
}

func structCallback(p Point) int64 {
	return int64(p.X) + int64(p.Y) + int64(p.Z) + int64(p.W)
}

func AddInts(a, b int32) int32 {
	var r int32
	if err := ffi.CallFunction(&addIntsCIF, addIntsSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)}); err != nil {
		panic(err)
	}
	return r
}

// Strlen passes a NUL-terminated Go string to bench_strlen. The trailing zero
// is appended explicitly so the C side observes a valid C string regardless
// of what the caller passed in.
func Strlen(s string) int {
	buf := append([]byte(s), 0)
	ptr := unsafe.Pointer(&buf[0])
	var r uint64
	if err := ffi.CallFunction(&strlenCIF, strlenSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{unsafe.Pointer(&ptr)}); err != nil {
		panic(err)
	}
	return int(r)
}

func SumBytes(data []byte) int64 {
	var ptr unsafe.Pointer
	if len(data) > 0 {
		ptr = unsafe.Pointer(&data[0])
	}
	length := uint64(len(data))
	var r int64
	if err := ffi.CallFunction(&sumBytesCIF, sumBytesSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{unsafe.Pointer(&ptr), unsafe.Pointer(&length)}); err != nil {
		panic(err)
	}
	return r
}

func PointAdd(a, b Point) Point {
	var r Point
	if err := ffi.CallFunction(&pointAddCIF, pointAddSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)}); err != nil {
		panic(err)
	}
	return r
}

func CallIntCallback(n, iters int32) int32 {
	cb := intCallbackPtr
	var r int32
	if err := ffi.CallFunction(&callIntCbCIF, callIntCbSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{
			unsafe.Pointer(&cb),
			unsafe.Pointer(&n),
			unsafe.Pointer(&iters),
		}); err != nil {
		panic(err)
	}
	return r
}

func CallStructCallback(p Point) int64 {
	cb := structCallbackPtr
	var r int64
	if err := ffi.CallFunction(&callStructCbCIF, callStructCbSym, unsafe.Pointer(&r),
		[]unsafe.Pointer{unsafe.Pointer(&cb), unsafe.Pointer(&p)}); err != nil {
		panic(err)
	}
	return r
}
