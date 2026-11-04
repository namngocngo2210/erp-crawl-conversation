package pancake

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimiter_PerPageThrottles(t *testing.T) {
	rl := NewRateLimiter(10, 1) // 10 rps, burst 1
	ctx := context.Background()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx, "p1")) // immediate (burst)
	require.NoError(t, rl.Wait(ctx, "p1")) // must wait ~100ms
	require.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
}

func TestRateLimiter_IndependentPerPage(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	ctx := context.Background()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx, "p1"))
	require.NoError(t, rl.Wait(ctx, "p2")) // different page, own bucket, immediate
	require.Less(t, time.Since(start), 200*time.Millisecond)
}
