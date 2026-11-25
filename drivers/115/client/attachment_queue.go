package pan115client

import (
	"context"
	"sync"
)

// Cookie ordinary attachment work uses the reference process-shared priority3
// single worker. All jobs here have the same priority, so FIFO is sufficient.
type attachmentJob struct {
	ctx  context.Context
	run  func() ([]byte, error)
	done chan attachmentResult
}
type attachmentResult struct {
	body []byte
	err  error
}

var attachmentQueue struct {
	once  sync.Once
	mu    sync.Mutex
	ready *sync.Cond
	jobs  []attachmentJob
}

func queueAttachment(ctx context.Context, run func() ([]byte, error)) ([]byte, error) {
	attachmentQueue.once.Do(func() {
		attachmentQueue.ready = sync.NewCond(&attachmentQueue.mu)
		go func() {
			for {
				attachmentQueue.mu.Lock()
				for len(attachmentQueue.jobs) == 0 {
					attachmentQueue.ready.Wait()
				}
				job := attachmentQueue.jobs[0]
				attachmentQueue.jobs[0] = attachmentJob{}
				attachmentQueue.jobs = attachmentQueue.jobs[1:]
				attachmentQueue.mu.Unlock()
				if e := job.ctx.Err(); e != nil {
					job.done <- attachmentResult{err: e}
					continue
				}
				body, e := job.run()
				job.done <- attachmentResult{body, e}
			}
		}()
	})
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	done := make(chan attachmentResult, 1)
	attachmentQueue.mu.Lock()
	attachmentQueue.jobs = append(attachmentQueue.jobs, attachmentJob{ctx, run, done})
	attachmentQueue.ready.Signal()
	attachmentQueue.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.body, result.err
	}
}
