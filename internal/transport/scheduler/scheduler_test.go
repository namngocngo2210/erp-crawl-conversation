package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrigger_NonOverlapping(t *testing.T) {
	var running sync.WaitGroup
	running.Add(1)
	release := make(chan struct{})
	var calls int32
	run := func(ctx context.Context, trigger string) error {
		atomic.AddInt32(&calls, 1)
		running.Done()
		<-release // hold the run open
		return nil
	}
	s, err := NewScheduler("0 21 * * *", "Asia/Ho_Chi_Minh", run, slog.Default())
	require.NoError(t, err)

	require.True(t, s.Trigger())  // starts
	running.Wait()                // ensure first run is in-flight
	require.False(t, s.Trigger()) // second rejected while busy
	close(release)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestNewScheduler_BadTimezone(t *testing.T) {
	_, err := NewScheduler("0 21 * * *", "Not/AZone", func(context.Context, string) error { return nil }, slog.Default())
	require.Error(t, err)
}
