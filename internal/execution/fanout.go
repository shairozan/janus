package execution

import (
	"context"
	"sync"
)

// fanResult is the outcome of processing one item in fanOut: its input index,
// the produced value, and any error. Errors are isolated per item — one item's
// failure never stops the others.
type fanResult[R any] struct {
	Index int
	Value R
	Err   error
}

// fanOut runs fn over items with at most parallelism workers active at once — a
// FIFO sliding window. Each worker pulls the next item the instant it finishes,
// so concurrency stays at the watermark until the queue drains (not
// wait-for-all-then-refill). This is the bootstrap fit pool: parallelism (P) is
// the hard ceiling on live fit pods / port-forwards / gRPC connections, and the
// cost/speed dial (#192).
//
// Results are returned in input order (results[i] corresponds to items[i]). A
// non-nil ctx error stops dispatching further items, which are recorded with
// that error; in-flight items observe ctx via fn. parallelism < 1 is treated as
// 1.
func fanOut[T, R any](
	ctx context.Context,
	items []T,
	parallelism int,
	fn func(ctx context.Context, index int, item T) (R, error),
) []fanResult[R] {
	if parallelism < 1 {
		parallelism = 1
	}

	results := make([]fanResult[R], len(items))
	sem := make(chan struct{}, parallelism)

	var wg sync.WaitGroup

	for i, item := range items {
		results[i].Index = i

		// Stop dispatching once the context is done; mark the rest cancelled.
		if err := ctx.Err(); err != nil {
			results[i].Err = err

			continue
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i].Err = ctx.Err()

			continue
		}

		wg.Add(1)

		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()

			v, err := fn(ctx, i, item)
			results[i].Value = v
			results[i].Err = err
		}(i, item)
	}

	wg.Wait()

	return results
}
