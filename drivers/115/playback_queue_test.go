package pan115

import (
	"context"
	"errors"
	"litepan/internal/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSTRMOpenQueueTwoWorkersFIFOAndCancellation(t *testing.T) {
	var q openPlaybackQueue
	started := make(chan int, 8)
	release := make(chan struct{})
	var active, max atomic.Int32
	var wg sync.WaitGroup
	launch := func(ctx context.Context, id int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = q.resolve(ctx, func(context.Context) (*domain.DownloadInfo, error) {
				n := active.Add(1)
				for {
					old := max.Load()
					if n <= old || max.CompareAndSwap(old, n) {
						break
					}
				}
				started <- id
				<-release
				active.Add(-1)
				return &domain.DownloadInfo{}, nil
			})
		}()
	}
	launch(context.Background(), 1)
	launch(context.Background(), 2)
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("two workers did not start")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	launch(cancelled, 3)
	deadline := time.Now().Add(time.Second)
	for {
		q.mu.Lock()
		pending := len(q.tasks)
		q.mu.Unlock()
		if pending == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task not queued")
		}
		time.Sleep(time.Millisecond)
	}
	launch(context.Background(), 4)
	for {
		q.mu.Lock()
		pending := len(q.tasks)
		q.mu.Unlock()
		if pending == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task not queued")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case id := <-started:
		t.Fatalf("third task ran before worker free: %d", id)
	default:
	}
	release <- struct{}{}
	select {
	case id := <-started:
		if id != 4 {
			t.Fatalf("cancelled task ran: %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("pending task did not run")
	}
	close(release)
	wg.Wait()
	if max.Load() != 2 {
		t.Fatalf("concurrency=%d", max.Load())
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	_, err := q.resolve(ctx, func(context.Context) (*domain.DownloadInfo, error) {
		t.Fatal("cancelled task executed")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestSTRMQueuePreservesOneSecondResultObservation(t *testing.T) {
	var q openPlaybackQueue
	started := make(chan struct{})
	finish := make(chan struct{})
	returned := make(chan error, 1)
	begin := time.Now()
	go func() {
		_, err := q.resolve(context.Background(), func(context.Context) (*domain.DownloadInfo, error) {
			close(started)
			<-finish
			return &domain.DownloadInfo{}, nil
		})
		returned <- err
	}()
	<-started
	// Keep completion unavailable during the immediate observation.
	time.Sleep(50 * time.Millisecond)
	close(finish)
	select {
	case err := <-returned:
		t.Fatalf("result observed before reference poll: elapsed=%v err=%v", time.Since(begin), err)
	case <-time.After(300 * time.Millisecond):
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(begin); elapsed < 900*time.Millisecond {
			t.Fatalf("early observation %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("result not observed")
	}
}
