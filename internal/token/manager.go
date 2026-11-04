package token

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenGenerator interface {
	GeneratePageToken(ctx context.Context, pageID string) (string, error)
}

type Manager interface {
	Get(ctx context.Context, pageID string) (string, error)
	Invalidate(ctx context.Context, pageID string) error
}

type manager struct {
	rdb   *redis.Client
	gen   TokenGenerator
	ttl   time.Duration
	locks sync.Map // pageID -> *sync.Mutex
}

func NewManager(rdb *redis.Client, gen TokenGenerator, ttl time.Duration) Manager {
	return &manager{rdb: rdb, gen: gen, ttl: ttl}
}

func key(pageID string) string { return "pancake:ptoken:" + pageID }

func (m *manager) lockFor(pageID string) *sync.Mutex {
	l, _ := m.locks.LoadOrStore(pageID, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (m *manager) Get(ctx context.Context, pageID string) (string, error) {
	if v, err := m.rdb.Get(ctx, key(pageID)).Result(); err == nil {
		return v, nil
	} else if !errors.Is(err, redis.Nil) {
		return "", err
	}
	mu := m.lockFor(pageID)
	mu.Lock()
	defer mu.Unlock()
	// double-check after acquiring lock
	if v, err := m.rdb.Get(ctx, key(pageID)).Result(); err == nil {
		return v, nil
	}
	tok, err := m.gen.GeneratePageToken(ctx, pageID)
	if err != nil {
		return "", err
	}
	if err := m.rdb.Set(ctx, key(pageID), tok, m.ttl).Err(); err != nil {
		return "", err
	}
	return tok, nil
}

func (m *manager) Invalidate(ctx context.Context, pageID string) error {
	return m.rdb.Del(ctx, key(pageID)).Err()
}
