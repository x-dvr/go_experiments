package fanin

import (
	"reflect"
	"sync"
)

func MergeCanonical[T any](inputs ...<-chan T) <-chan T {
	outCh := make(chan T, len(inputs))
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	// create data transfer goroutine for each input channel
	for _, ch := range inputs {
		go func() {
			defer wg.Done()
			for val := range ch {
				outCh <- val
			}
		}()
	}

	// wait for all goroutines to finish, then close the channel
	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func MergeReflect[T any](inputs ...<-chan T) <-chan T {
	outCh := make(chan T, len(inputs))
	cases := make([]reflect.SelectCase, len(inputs))

	// create reflect.SelectCase for each input channel
	for i, ch := range inputs {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	go func() {
		defer close(outCh)

		// keep selecting until all channels are closed
		for len(cases) > 0 {
			idx, val, ok := reflect.Select(cases)
			if !ok {
				// channel is closed, remove it's case
				cases = append(cases[:idx], cases[idx+1:]...)
				continue
			}
			v, _ := val.Interface().(T)
			outCh <- v
		}
	}()

	return outCh
}

func MergeLoop[T any](inputs ...<-chan T) <-chan T {
	outCh := make(chan T, len(inputs))
	total := len(inputs)
	active := total

	go func() {
		// iterate while we have at least one open channel
		for active > 0 {
			for idx := 0; idx < active; idx++ {
				select {
				case val, ok := <-inputs[idx]:
					if !ok {
						// channel is closed, remove it from inputs and decrease active count
						active--
						inputs = append(inputs[:idx], inputs[idx+1:]...)
						continue
					}
					outCh <- val
				default:
				}
			}
		}
		close(outCh)
	}()

	return outCh
}
