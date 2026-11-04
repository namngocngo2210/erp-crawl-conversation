package read

import (
	"context"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

type MsgSearchFilter struct {
	Query     string
	PageID    int64
	Direction string
	From, To  *time.Time
	Cursor    string
	Limit     int
}

type CustomerFilter struct {
	PageID int64
	Query  string
	Cursor string
	Limit  int
}

type SearchRepository interface {
	SearchMessages(ctx context.Context, f MsgSearchFilter) ([]entity.Message, error)
	SearchCustomers(ctx context.Context, f CustomerFilter) ([]entity.Customer, error)
}

type searchRepo struct{ db *repository.DB }

func NewSearchRepository(db *repository.DB) SearchRepository { return &searchRepo{db} }

func (r *searchRepo) SearchMessages(ctx context.Context, f MsgSearchFilter) ([]entity.Message, error) {
	q := r.db.Gorm.WithContext(ctx).
		Model(&entity.Message{}).
		Where("content_tsv @@ plainto_tsquery('simple', ?)", f.Query).
		Order("inserted_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if f.PageID > 0 {
		q = q.Where("page_id = ?", f.PageID)
	}
	if f.Direction == "in" || f.Direction == "out" {
		q = q.Where("direction = ?", f.Direction)
	}
	if f.From != nil {
		q = q.Where("inserted_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("inserted_at <= ?", *f.To)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(inserted_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Message
	err := q.Find(&out).Error
	return out, err
}

func (r *searchRepo) SearchCustomers(ctx context.Context, f CustomerFilter) ([]entity.Customer, error) {
	q := r.db.Gorm.WithContext(ctx).
		Model(&entity.Customer{}).
		Order("id DESC").
		Limit(clampLimit(f.Limit))
	if f.PageID > 0 {
		q = q.Where("page_id = ?", f.PageID)
	}
	if f.Query != "" {
		like := "%" + f.Query + "%"
		q = q.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ?", like, like, like)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("id < ?", c.ID)
	}
	var out []entity.Customer
	err := q.Find(&out).Error
	return out, err
}
