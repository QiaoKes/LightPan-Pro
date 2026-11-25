package pan115client

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"time"
)

// referencePageWorkers follows concurrenttools' default CPU-based worker count,
// shared increasing page allocator, minimum terminal page, and completion order.
func referencePageWorkers(ctx context.Context, fetch func(context.Context, int) ([]byte, bool, error), emit func([]byte) error) error {
	ctx, cancel := context.WithCancel(ctx)
	workers := min(32, runtime.NumCPU()+4)
	type result struct {
		body []byte
		err  error
	}
	results := make(chan result, workers)
	var mu sync.Mutex
	next, maxPage := 1, 0
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				page := next
				next++
				stop := ctx.Err() != nil || (maxPage > 0 && page > maxPage)
				mu.Unlock()
				if stop {
					return
				}
				body, terminal, err := fetch(ctx, page)
				if err != nil && referencePagingRetry(ctx, err) {
					// Retry the same allocated page before asking for another.
					for err != nil && referencePagingRetry(ctx, err) {
						body, terminal, err = fetch(ctx, page)
					}
				}
				if terminal {
					mu.Lock()
					if maxPage == 0 || page < maxPage {
						maxPage = page
					}
					mu.Unlock()
				}
				select {
				case results <- result{body, err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()
	defer func() { cancel(); wg.Wait() }()
	for item := range results {
		if item.err != nil {
			return item.err
		}
		if err := emit(item.body); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// referenceOffsetPages mirrors the bundled deque of futures: start one call,
// rotate unfinished futures on the cooldown timeout, then speculate one offset.
func referenceOffsetPages(ctx context.Context, size int, cooldown time.Duration, fetch func(context.Context, int) ([]byte, bool, error), emit func([]byte) error) error {
	ctx, cancel := context.WithCancel(ctx)
	type result struct {
		body     []byte
		terminal bool
		err      error
	}
	type future struct {
		offset int
		done   chan result
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, min(32, runtime.NumCPU()+4))
	var last time.Time
	submit := func(offset int) future {
		last = time.Now()
		f := future{offset, make(chan result, 1)}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				f.done <- result{err: ctx.Err()}
				return
			}
			body, end, err := fetch(ctx, offset)
			f.done <- result{body, end, err}
		}()
		return f
	}
	queue := []future{submit(0)}
	next, maxOffset := size, -1
	defer func() { cancel(); wg.Wait() }()
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		var r result
		ready := false
		select {
		case r = <-f.done:
			ready = true
		default:
		}
		if !ready {
			timer := time.NewTimer(max(time.Duration(0), time.Until(last.Add(cooldown))))
			select {
			case r = <-f.done:
				ready = true
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
			timer.Stop()
		}
		if ready {
			if r.err != nil {
				if !referencePagingRetry(ctx, r.err) {
					return r.err
				}
				queue = append(queue, submit(f.offset))
				if maxOffset < 0 {
					queue = append(queue, submit(next))
					next += size
				}
				continue
			}
			if err := emit(r.body); err != nil {
				return err
			}
			if r.terminal && (maxOffset < 0 || f.offset < maxOffset) {
				maxOffset = f.offset
				kept := queue[:0]
				for _, pending := range queue {
					if pending.offset <= maxOffset {
						kept = append(kept, pending)
					}
				}
				queue = kept
			}
		} else {
			queue = append(queue, f)
		}
		if maxOffset < 0 {
			queue = append(queue, submit(next))
			next += size
		}
	}
	return ctx.Err()
}

type pagingStopError struct{ error }

func referencePagingRetry(ctx context.Context, err error) bool {
	var stop *pagingStopError
	if errors.As(err, &stop) {
		return false
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return true
	}
	var status *HTTPError
	if errors.As(err, &status) {
		return status.StatusCode < 400
	}
	var api *APIError
	if errors.As(err, &api) {
		return api.ResponseStatus < 400
	}
	return true
}
