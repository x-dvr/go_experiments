package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/trace"
	"sync"

	workerpool "github.com/x-dvr/go_experiments/worker_pool"
)

const (
	CalcTo   int = 1e5
	RunTimes int = 1e4
)

var (
	PoolCap int   = runtime.NumCPU() * 2
	sink    []int = make([]int, RunTimes)
)

func main() {
	pools := map[string]func(){
		"no": noPool,
		"rr": robinPool,
		"st": staticPool,
	}
	which := "all"
	if len(os.Args) > 1 {
		which = os.Args[1]
	}
	if which == "all" {
		for name, fn := range pools {
			if err := traceFn(name, fn); err != nil {
				panic(err)
			}
		}
		return
	}
	fn, ok := pools[which]
	if !ok {
		panic("unknown pool: " + which)
	}
	if err := traceFn(which, fn); err != nil {
		panic(err)
	}
}

func traceFn(name string, fn func()) error {
	f, err := os.Create(fmt.Sprintf("trace_%s.out", name))
	if err != nil {
		return err
	}
	defer f.Close()

	err = trace.Start(f)
	if err != nil {
		return err
	}
	defer trace.Stop()
	fn()
	return nil
}

func noPool() {
	var wg sync.WaitGroup
	for i := range RunTimes {
		wg.Go(func() {
			sink[i] = workHard(CalcTo)
		})
	}
	wg.Wait()
}

func robinPool() {
	pool := workerpool.NewRRPool(func(ct, i int) {
		sink[i] = workHard(ct)
	}, int64(PoolCap))
	for i := range RunTimes {
		pool.Go(CalcTo, i)
	}
	pool.Drain()
	pool.Release()
}

func staticPool() {
	pool := workerpool.NewStaticPool(func(ct, i int) {
		sink[i] = workHard(ct)
	}, PoolCap)
	for i := range RunTimes {
		pool.Go(CalcTo, i)
	}
	pool.Drain()
	pool.Release()
}

func workHard(calcTo int) int {
	var n2, n1 = 0, 1
	for i := 2; i <= calcTo; i++ {
		n2, n1 = n1, n1+n2
	}
	return n1
}
