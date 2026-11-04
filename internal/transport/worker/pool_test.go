package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_AllExecuteAndCollectErrors(t *testing.T) {
	var count int32
	jobs := []Job{
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return nil },
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return errors.New("boom") },
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return nil },
	}
	errs := Run(context.Background(), 2, jobs)
	require.Equal(t, int32(3), count)
	var failures int
	for _, e := range errs {
		if e != nil {
			failures++
		}
	}
	require.Equal(t, 1, failures)
}

func TestRun_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so ctx.Done() is already closed

	block := make(chan struct{}) // never sends - jobs would block forever if run
	jobs := make([]Job, 5)
	for i := range jobs {
		jobs[i] = func(ctx context.Context) error {
			<-block // blocks forever unless interrupted
			return nil
		}
	}

	// Should return promptly because ctx is already cancelled at dispatch time.
	errs := Run(ctx, 2, jobs)

	require.Len(t, errs, 5)
	for i, err := range errs {
		require.NotNil(t, err, "expected non-nil error for job %d", i)
		require.ErrorIs(t, err, context.Canceled, "expected context.Canceled for job %d", i)
	}
}
