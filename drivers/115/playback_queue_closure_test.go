package pan115

import (
	"context"
	"litepan/internal/domain"
	"testing"
)

func TestClosureQueueSurvivingFIFOAndPriority(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var order []int
	q := openPlaybackQueue{workers: 1}
	add := func(id, priority int, sequence uint64, c context.Context) {
		q.tasks = append(q.tasks, &playbackQueueTask{priority: priority, sequence: sequence, ctx: c, result: make(chan playbackQueueResult, 1), run: func(context.Context) (*domain.DownloadInfo, error) { order = append(order, id); return nil, nil }})
	}
	// Heap-valid insertion order: cancelled high priority, then same-priority FIFO,
	// with a later lower-priority task that must run last.
	add(0, 1, 1, ctx)
	add(1, 2, 2, context.Background())
	add(2, 2, 3, context.Background())
	add(3, 3, 4, context.Background())
	q.worker()
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("execution=%v", order)
	}
}
