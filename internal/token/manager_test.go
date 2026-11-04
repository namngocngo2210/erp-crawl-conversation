package token

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type fakeGen struct{ calls int }

func (f *fakeGen) GeneratePageToken(ctx context.Context, pageID string) (string, error) {
	f.calls++
	return "tok-" + pageID, nil
}

func TestGet_CachesAfterFirstCall(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	gen := &fakeGen{}
	m := NewManager(rdb, gen, time.Hour)

	ctx := context.Background()
	tok1, err := m.Get(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "tok-p1", tok1)
	tok2, err := m.Get(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "tok-p1", tok2)
	require.Equal(t, 1, gen.calls) // second served from cache
}

func TestInvalidate(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	gen := &fakeGen{}
	m := NewManager(rdb, gen, time.Hour)
	ctx := context.Background()
	_, _ = m.Get(ctx, "p1")
	require.NoError(t, m.Invalidate(ctx, "p1"))
	_, _ = m.Get(ctx, "p1")
	require.Equal(t, 2, gen.calls) // regenerated after invalidate
}
