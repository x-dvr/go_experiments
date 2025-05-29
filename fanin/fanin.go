package fanin

import (
	"reflect"
	"sync"
)

func MergeCanonical[T any](inputs ...<-chan T) <-chan T {
	outCh := make(chan T, len(inputs))
	wg := &sync.WaitGroup{}
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

func MergeBatch4[T any](inputs ...<-chan T) <-chan T {
	inputCount := len(inputs)
	// number of goroutines to spawn
	gCount := inputCount / 4
	// number of channels handled by last goroutine
	leftovers := inputCount - (gCount * 4)

	outCh := make(chan T, inputCount)
	wg := &sync.WaitGroup{}
	wg.Add(gCount)

	// create data transfer goroutine for each 4 input channels
	for i := range gCount {
		go select4(wg, outCh, inputs[i*4], inputs[i*4+1], inputs[i*4+2], inputs[i*4+3])
	}

	switch leftovers {
	case 3:
		wg.Add(1)
		go select3(wg, outCh, inputs[inputCount-3], inputs[inputCount-2], inputs[inputCount-1])
	case 2:
		wg.Add(1)
		go select2(wg, outCh, inputs[inputCount-2], inputs[inputCount-1])
	case 1:
		wg.Add(1)
		go read(wg, outCh, inputs[inputCount-1])
	}

	// wait for all goroutines to finish, then close the channel
	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func MergeBatch2[T any](inputs ...<-chan T) <-chan T {
	inputCount := len(inputs)
	// number of goroutines to spawn
	gCount := inputCount / 2
	hasLeftover := inputCount%2 != 0

	outCh := make(chan T, inputCount)
	wg := &sync.WaitGroup{}
	wg.Add(gCount)

	// create data transfer goroutine for each 4 input channels
	for i := range gCount {
		go select2(wg, outCh, inputs[i*2], inputs[i*2+1])
	}

	if hasLeftover {
		wg.Add(1)
		go read(wg, outCh, inputs[inputCount-1])
	}

	// wait for all goroutines to finish, then close the channel
	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func select4[T any](wg *sync.WaitGroup, out chan<- T, ch1, ch2, ch3, ch4 <-chan T) {
	defer wg.Done()
	// loop while we have at least one open channel
	for ch1 != nil || ch2 != nil || ch3 != nil || ch4 != nil {
		select {
		case val, ok := <-ch1:
			if !ok {
				ch1 = nil
				continue
			}
			out <- val
		case val, ok := <-ch2:
			if !ok {
				ch2 = nil
				continue
			}
			out <- val
		case val, ok := <-ch3:
			if !ok {
				ch3 = nil
				continue
			}
			out <- val
		case val, ok := <-ch4:
			if !ok {
				ch4 = nil
				continue
			}
			out <- val
		}
	}
}

func select3[T any](wg *sync.WaitGroup, out chan<- T, ch1, ch2, ch3 <-chan T) {
	defer wg.Done()
	// loop while we have at least one open channel
	for ch1 != nil || ch2 != nil || ch3 != nil {
		select {
		case val, ok := <-ch1:
			if !ok {
				ch1 = nil
				continue
			}
			out <- val
		case val, ok := <-ch2:
			if !ok {
				ch2 = nil
				continue
			}
			out <- val
		case val, ok := <-ch3:
			if !ok {
				ch3 = nil
				continue
			}
			out <- val
		}
	}
}

func select2[T any](wg *sync.WaitGroup, out chan<- T, ch1, ch2 <-chan T) {
	defer wg.Done()
	// loop while we have at least one open channel
	for ch1 != nil || ch2 != nil {
		select {
		case val, ok := <-ch1:
			if !ok {
				ch1 = nil
				continue
			}
			out <- val
		case val, ok := <-ch2:
			if !ok {
				ch2 = nil
				continue
			}
			out <- val
		}
	}
}

func read[T any](wg *sync.WaitGroup, out chan<- T, in <-chan T) {
	defer wg.Done()
	for val := range in {
		out <- val
	}
}
