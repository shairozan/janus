package execution

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanOutPreservesOrderAndValues(t *testing.T) {
	items := []int{10, 20, 30, 40, 50}

	results := fanOut(context.Background(), items, 2, func(_ context.Context, i int, item int) (int, error) {
		return item + i, nil
	})

	if len(results) != len(items) {
		t.Fatalf("got %d results, want %d", len(results), len(items))
	}

	for i, item := range items {
		if results[i].Err != nil {
			t.Errorf("item %d: unexpected error %v", i, results[i].Err)
		}

		if want := item + i; results[i].Value != want {
			t.Errorf("results[%d].Value = %d, want %d", i, results[i].Value, want)
		}

		if results[i].Index != i {
			t.Errorf("results[%d].Index = %d", i, results[i].Index)
		}
	}
}

func TestFanOutRespectsParallelismBound(t *testing.T) {
	const p = 3

	var (
		active  int32
		maxSeen int32
	)

	items := make([]int, 30)

	fanOut(context.Background(), items, p, func(_ context.Context, _ int, _ int) (struct{}, error) {
		cur := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if cur <= old || atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
				break
			}
		}

		time.Sleep(2 * time.Millisecond) // hold the slot so overlap is observable
		atomic.AddInt32(&active, -1)

		return struct{}{}, nil
	})

	if maxSeen > p {
		t.Errorf("observed %d concurrent workers, exceeds parallelism %d", maxSeen, p)
	}

	if maxSeen < 2 {
		t.Errorf("expected real concurrency, only observed max %d", maxSeen)
	}
}

func TestFanOutIsolatesErrors(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5}
	errBoom := errors.New("boom")

	var ran int32

	results := fanOut(context.Background(), items, 4, func(_ context.Context, i int, _ int) (int, error) {
		atomic.AddInt32(&ran, 1)
		if i%2 == 0 {
			return 0, errBoom
		}

		return i, nil
	})

	if ran != int32(len(items)) {
		t.Errorf("expected all %d items to run, got %d", len(items), ran)
	}

	for i := range items {
		if i%2 == 0 {
			if !errors.Is(results[i].Err, errBoom) {
				t.Errorf("item %d: want error, got %v", i, results[i].Err)
			}
		} else {
			if results[i].Err != nil || results[i].Value != i {
				t.Errorf("item %d: want value %d no error, got %d/%v", i, i, results[i].Value, results[i].Err)
			}
		}
	}
}

func TestFanOutCancelledContextSkipsWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	var ran int32

	results := fanOut(ctx, []int{1, 2, 3}, 2, func(_ context.Context, _ int, _ int) (int, error) {
		atomic.AddInt32(&ran, 1)

		return 0, nil
	})

	if ran != 0 {
		t.Errorf("expected no work on cancelled context, ran %d", ran)
	}

	for i := range results {
		if !errors.Is(results[i].Err, context.Canceled) {
			t.Errorf("results[%d].Err = %v, want context.Canceled", i, results[i].Err)
		}
	}
}

func TestFanOutParallelismFloor(t *testing.T) {
	// parallelism < 1 is treated as 1 (still serial, still completes).
	var (
		mu      sync.Mutex
		active  int
		maxSeen int
	)

	fanOut(context.Background(), make([]int, 5), 0, func(_ context.Context, _ int, _ int) (struct{}, error) {
		mu.Lock()
		active++
		if active > maxSeen {
			maxSeen = active
		}
		mu.Unlock()

		time.Sleep(time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()

		return struct{}{}, nil
	})

	if maxSeen != 1 {
		t.Errorf("parallelism floor: max concurrency = %d, want 1", maxSeen)
	}
}

func TestFanOutEmpty(t *testing.T) {
	results := fanOut(context.Background(), []int{}, 4, func(_ context.Context, _ int, _ int) (int, error) {
		return 0, nil
	})

	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}
