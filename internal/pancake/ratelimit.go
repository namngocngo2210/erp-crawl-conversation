package pancake

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rps      rate.Limit
	burst    int
	limiters sync.Map // pageID -> *rate.Limiter
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{rps: rate.Limit(rps), burst: burst}
}

func (r *RateLimiter) Wait(ctx context.Context, pageID string) error {
	l, _ := r.limiters.LoadOrStore(pageID, rate.NewLimiter(r.rps, r.burst))
	return l.(*rate.Limiter).Wait(ctx)
}
