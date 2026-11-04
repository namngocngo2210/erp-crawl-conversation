package worker

import (
	"context"
	"fmt"
	"sync"
)

type Job func(ctx context.Context) error

func Run(ctx context.Context, size int, jobs []Job) []error {
	if size < 1 {
		size = 1
	}
	errs := make([]error, len(jobs))
	type item struct {
		idx int
		job Job
	}
	ch := make(chan item)
	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				if ctx.Err() != nil {
					errs[it.idx] = ctx.Err()
					continue
				}
				errs[it.idx] = runOne(ctx, it.job)
			}
		}()
	}
outer:
	for i, j := range jobs {
		select {
		case ch <- item{idx: i, job: j}:
		case <-ctx.Done():
			for k := i; k < len(jobs); k++ {
				errs[k] = ctx.Err()
			}
			break outer
		}
	}
	close(ch)
	wg.Wait()
	return errs
}

func runOne(ctx context.Context, job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return job(ctx)
}
