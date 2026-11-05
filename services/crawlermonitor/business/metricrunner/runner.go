package metricrunner

import (
	"context"
	"sync"

	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/entity"
)

// Call produces one metric result. Calls are the unit the Runner schedules.
type Call func(context.Context) entity.Result

type Runner struct {
	concurrency int
}

func NewRunner(concurrency int) *Runner {
	if concurrency < 1 {
		concurrency = 1
	}

	return &Runner{
		concurrency: concurrency,
	}
}

func (r *Runner) Run(
	ctx context.Context,
	calls []Call,
) []entity.Result {
	results := make([]entity.Result, len(calls))
	if len(calls) == 0 {
		return results
	}

	limit := min(r.concurrency, len(calls))

	semaphore := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, call := range calls {
		if ctx.Err() != nil {
			break
		}

		semaphore <- struct{}{}
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()

			results[i] = call(ctx)
		}()
	}

	wg.Wait()
	return results
}