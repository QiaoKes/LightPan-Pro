package pan115client

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReferencePagesUseCPUWorkersAndCompletionOrder(t *testing.T) {
	workers := min(32, runtime.NumCPU()+4)
	var mu sync.Mutex
	started := map[int]bool{}
	all := make(chan struct{})
	stop := fmt.Errorf("test stop")
	err := referencePageWorkers(context.Background(), func(ctx context.Context, page int) ([]byte, bool, error) {
		mu.Lock()
		started[page] = true
		if len(started) == workers {
			close(all)
		}
		mu.Unlock()
		select {
		case <-all:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
		if page != 2 {
			<-ctx.Done()
			return nil, false, ctx.Err()
		}
		return []byte("page2"), true, nil
	}, func(body []byte) error {
		if string(body) != "page2" {
			t.Errorf("wrong first completion %q", body)
		}
		return stop
	})
	if err != stop || len(started) != workers {
		t.Fatal(err, len(started), workers)
	}
}

func TestReferenceOffsetsSpeculateOnTimeoutAndDrain(t *testing.T) {
	firstStopped := make(chan struct{})
	secondStarted := make(chan struct{})
	var offsets []int
	var mu sync.Mutex
	err := referenceOffsetPages(context.Background(), 7000, 5*time.Millisecond, func(ctx context.Context, offset int) ([]byte, bool, error) {
		mu.Lock()
		offsets = append(offsets, offset)
		mu.Unlock()
		if offset == 0 {
			<-secondStarted
			time.Sleep(15 * time.Millisecond)
			close(firstStopped)
			return []byte("first"), false, nil
		}
		if offset == 7000 {
			close(secondStarted)
		}
		return []byte("later"), true, nil
	}, func(body []byte) error {
		if string(body) != "later" && string(body) != "first" {
			t.Errorf("wrong completion %s", body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStopped:
	default:
		t.Fatal("first request not drained")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(offsets) < 2 || offsets[0] != 0 || offsets[1] != 7000 {
		t.Fatal(offsets)
	}
}

func TestReferencePagingRetriesSamePageOnlyForRetryableErrors(t *testing.T) {
	var attempts atomic.Int64
	err := referenceOffsetPages(context.Background(), 1, time.Millisecond, func(context.Context, int) ([]byte, bool, error) {
		if attempts.Add(1) == 1 {
			return nil, false, fmt.Errorf("connection reset")
		}
		return []byte("ok"), true, nil
	}, func([]byte) error { return nil })
	if err != nil || attempts.Load() < 2 {
		t.Fatal(attempts.Load(), err)
	}
	if referencePagingRetry(context.Background(), &HTTPError{StatusCode: 403}) {
		t.Fatal("HTTP denial retried")
	}
}

func TestReferenceRetryStatusAndCancellation(t *testing.T) {
	for _, raw := range []string{`{"state":false,"errno":99,"code":401}`, `{"state":false,"status":403,"code":1}`} {
		if err := CheckResponse([]byte(raw)); referencePagingRetry(context.Background(), err) {
			t.Fatalf("retry denial %s", raw)
		}
	}
	if referencePagingRetry(context.Background(), &HTTPError{StatusCode: 401}) {
		t.Fatal("retry HTTP401")
	}
	if !referencePagingRetry(context.Background(), CheckResponse([]byte(`{"state":false,"errno":99}`))) {
		t.Fatal("missing status must retain SDK retry")
	}
	ctx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int64
	err := referenceOffsetPages(ctx, 1, time.Hour, func(context.Context, int) ([]byte, bool, error) {
		attempts.Add(1)
		cancel()
		return nil, false, fmt.Errorf("connection reset")
	}, func([]byte) error { return nil })
	if err == nil || attempts.Load() != 1 {
		t.Fatal(err, attempts.Load())
	}
}

func TestDirectoryCacheRequiresCompleteUIDChain(t *testing.T) {
	cacheDirectoryNodes("chain-a", map[string]cachedDirNode{"3": {"Leaf", "2"}})
	if _, ok := cachedDirectoryPath("chain-a", "3"); ok {
		t.Fatal("incomplete ancestry hit")
	}
	cacheDirectoryNodes("chain-a", map[string]cachedDirNode{"2": {"Parent", "0"}})
	if got, ok := cachedDirectoryPath("chain-a", "3"); !ok || got != "/Parent/Leaf" {
		t.Fatal(got, ok)
	}
	if _, ok := cachedDirectoryPath("chain-b", "3"); ok {
		t.Fatal("cross-UID hit")
	}
	cacheDirectoryNodes("chain-a", map[string]cachedDirNode{"2": {"Renamed", "0"}})
	if got, _ := cachedDirectoryPath("chain-a", "3"); got != "/Renamed/Leaf" {
		t.Fatal(got)
	}
}
