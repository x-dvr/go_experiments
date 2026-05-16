#ifndef FFILIB_H
#define FFILIB_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int32_t bench_add_ints(int32_t a, int32_t b);

size_t bench_strlen(const char *s);

int64_t bench_sum_bytes(const uint8_t *data, size_t len);

typedef struct {
    int32_t x;
    int32_t y;
    int32_t z;
    int32_t w;
} BenchPoint;

BenchPoint bench_point_add(BenchPoint a, BenchPoint b);

typedef int32_t (*bench_int_callback_t)(int32_t);
int32_t bench_call_int_callback(bench_int_callback_t cb, int32_t n, int32_t iters);

typedef int64_t (*bench_struct_callback_t)(BenchPoint);
int64_t bench_call_struct_callback(bench_struct_callback_t cb, BenchPoint p);

#ifdef __cplusplus
}
#endif

#endif
