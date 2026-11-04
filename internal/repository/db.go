package repository

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	Gorm     *gorm.DB
	Pool     *pgxpool.Pool
	partDone sync.Map // month string -> struct{}
}

func New(ctx context.Context, dsn string) (*DB, error) {
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &DB{Gorm: g, Pool: pool}, nil
}

func (d *DB) Close() {
	d.Pool.Close()
	if sqlDB, err := d.Gorm.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func (d *DB) EnsurePartition(ctx context.Context, ts time.Time) error {
	month := time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.UTC)
	key := month.Format("2006-01")
	if _, ok := d.partDone.Load(key); ok {
		return nil
	}
	if _, err := d.Pool.Exec(ctx, "SELECT ensure_messages_partition($1)", month); err != nil {
		return err
	}
	d.partDone.Store(key, struct{}{})
	return nil
}
