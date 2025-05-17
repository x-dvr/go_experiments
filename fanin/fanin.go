package fanin

import (
	"iter"
	"reflect"
	"sync"
)

func MergeGoChan[T any](chans ...<-chan T) <-chan T {
	var wg sync.WaitGroup
	wg.Add(len(chans))

	outCh := make(chan T)
	for _, ch := range chans {
		go func() {
			defer wg.Done()
			for val := range ch {
				outCh <- val
			}
		}()
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func MergeLoopIter[T any](chans ...<-chan T) iter.Seq[T] {
	total := len(chans)
	var aborted bool
	type void struct{}
	var token void
	closedCh := make(map[int]void, total)

	return func(yield func(T) bool) {
		for len(closedCh) < total {
			for idx, ch := range chans {
				if _, closed := closedCh[idx]; closed {
					continue
				}
				select {
				case val, ok := <-ch:
					if !ok {
						closedCh[idx] = token
						continue
					}
					if !aborted {
						if !yield(val) {
							aborted = true
							return
						}
					}
				default:
				}
			}
		}
	}
}

func MergeReflectChan[T any](chans ...<-chan T) <-chan T {
	outCh := make(chan T, len(chans))
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	go func() {
		defer close(outCh)
		for len(cases) > 0 {
			idx, val, ok := reflect.Select(cases)
			if !ok {
				cases = append(cases[:idx], cases[idx+1:]...)
			}
			v, _ := val.Interface().(T)
			outCh <- v
		}
	}()

	return outCh
}

func MergeReflectIter[T any](chans ...<-chan T) iter.Seq[T] {
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}

	return func(yield func(T) bool) {
		for len(cases) > 0 {
			idx, val, ok := reflect.Select(cases)
			if !ok {
				cases = append(cases[:idx], cases[idx+1:]...)
			}
			v, _ := val.Interface().(T)
			if !yield(v) {
				return
			}
		}
	}
}
