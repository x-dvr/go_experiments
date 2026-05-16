package static

import (
	"testing"
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

func BenchmarkStrlenNoCopy(b *testing.B) {
	var r int
	for i := 0; i < b.N; i++ {
		r = StrlenNoCopy(benchStr)
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
