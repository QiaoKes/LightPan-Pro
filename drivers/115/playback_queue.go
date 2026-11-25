package pan115

import (
	"container/heap"
	"context"
	"strings"
	"sync"
	"time"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

type playbackQueueResult struct {
	link *domain.DownloadInfo
	err  error
}
type playbackQueueTask struct {
	priority int
	sequence uint64
	ctx      context.Context
	run      func(context.Context) (*domain.DownloadInfo, error)
	result   chan playbackQueueResult
}
type playbackTaskHeap []*playbackQueueTask

func (h playbackTaskHeap) Len() int { return len(h) }
func (h playbackTaskHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority < h[j].priority
	}
	return h[i].sequence < h[j].sequence
}
func (h playbackTaskHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *playbackTaskHeap) Push(v any)   { *h = append(*h, v.(*playbackQueueTask)) }
func (h *playbackTaskHeap) Pop() any {
	old := *h
	n := len(old)
	v := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return v
}

type openPlaybackQueue struct {
	mu       sync.Mutex
	tasks    playbackTaskHeap
	sequence uint64
	workers  int
}

// Reference constant.py creates one process-shared two-worker Open queue.
// The existing Open API limiter remains the sole business-call cadence gate.
var strmOpenPlaybackQueue openPlaybackQueue

func (q *openPlaybackQueue) resolve(ctx context.Context, run func(context.Context) (*domain.DownloadInfo, error)) (*domain.DownloadInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	task := &playbackQueueTask{priority: 2, ctx: ctx, run: run, result: make(chan playbackQueueResult, 1)}
	q.mu.Lock()
	q.sequence++
	task.sequence = q.sequence
	heap.Push(&q.tasks, task)
	if q.workers < 2 {
		q.workers++
		go q.worker()
	}
	q.mu.Unlock()

	// strm_get_result observes immediately, then at one-second intervals.
	// Completion does not itself wake the waiter between those observations.
	select {
	case result := <-task.result:
		return result.link, result.err
	default:
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			select {
			case result := <-task.result:
				return result.link, result.err
			default:
			}
		}
	}
}
func (q *openPlaybackQueue) worker() {
	for {
		q.mu.Lock()
		if len(q.tasks) == 0 {
			q.workers--
			q.mu.Unlock()
			return
		}
		task := heap.Pop(&q.tasks).(*playbackQueueTask)
		q.mu.Unlock()
		if err := task.ctx.Err(); err != nil {
			task.result <- playbackQueueResult{err: err}
			continue
		}
		link, err := task.run(task.ctx)
		task.result <- playbackQueueResult{link: link, err: err}
	}
}
func (d *Driver) ResolveSTRMDownload(ctx context.Context, req driver.DownloadRequest) (*domain.DownloadInfo, error) {
	if !strings.Contains(req.UA, "Lavf") {
		return d.ResolveDownload(ctx, req)
	}
	return strmOpenPlaybackQueue.resolve(ctx, func(ctx context.Context) (*domain.DownloadInfo, error) { return d.ResolveDownload(ctx, req) })
}
