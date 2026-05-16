#include "ffilib.h"

int32_t bench_add_ints(int32_t a, int32_t b) {
    return a + b;
}

size_t bench_strlen(const char *s) {
    const char *p = s;
    while (*p) p++;
    return (size_t)(p - s);
}

int64_t bench_sum_bytes(const uint8_t *data, size_t len) {
    int64_t sum = 0;
    for (size_t i = 0; i < len; i++) {
        sum += data[i];
    }
    return sum;
}

BenchPoint bench_point_add(BenchPoint a, BenchPoint b) {
    BenchPoint r = {a.x + b.x, a.y + b.y, a.z + b.z, a.w + b.w};
    return r;
}

int32_t bench_call_int_callback(bench_int_callback_t cb, int32_t n, int32_t iters) {
    int32_t acc = n;
    for (int32_t i = 0; i < iters; i++) {
        acc = cb(acc);
    }
    return acc;
}

int64_t bench_call_struct_callback(bench_struct_callback_t cb, BenchPoint p) {
    return cb(p);
}
