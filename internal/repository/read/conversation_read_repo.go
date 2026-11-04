package read

import (
	"context"
	"errors"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"gorm.io/gorm"
)

type PageCount struct {
	entity.Page
	ConversationCount int64
}

type ConvFilter struct {
	PageID int64
	Cursor string
	Limit  int
}

type MsgFilter struct {
	ConversationID int64
	Direction      string
	Cursor         string
	Limit          int
}

type ConvDetail struct {
	Conversation entity.Conversation
	Customer     *entity.Customer
	Tags         []entity.Tag
}

type ReadRepository interface {
	ListPages(ctx context.Context) ([]PageCount, error)
	ListConversations(ctx context.Context, f ConvFilter) ([]entity.Conversation, error)
	GetConversation(ctx context.Context, id int64) (ConvDetail, error)
	ListMessages(ctx context.Context, f MsgFilter) ([]entity.Message, error)
}

type readRepo struct{ db *repository.DB }

func NewReadRepository(db *repository.DB) ReadRepository { return &readRepo{db} }

func clampLimit(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}

func (r *readRepo) ListPages(ctx context.Context) ([]PageCount, error) {
	var out []PageCount
	err := r.db.Gorm.WithContext(ctx).
		Model(&entity.Page{}).
		Select("pages.*, (SELECT count(*) FROM conversations c WHERE c.page_id = pages.id) AS conversation_count").
		Order("pages.name ASC").
		Find(&out).Error
	return out, err
}

func (r *readRepo) ListConversations(ctx context.Context, f ConvFilter) ([]entity.Conversation, error) {
	q := r.db.Gorm.WithContext(ctx).
		Where("page_id = ?", f.PageID).
		Order("updated_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(updated_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Conversation
	err := q.Find(&out).Error
	return out, err
}

func (r *readRepo) GetConversation(ctx context.Context, id int64) (ConvDetail, error) {
	var conv entity.Conversation
	if err := r.db.Gorm.WithContext(ctx).First(&conv, id).Error; err != nil {
		return ConvDetail{}, err
	}
	detail := ConvDetail{Conversation: conv}
	if conv.CustomerID != nil {
		var cust entity.Customer
		if err := r.db.Gorm.WithContext(ctx).First(&cust, *conv.CustomerID).Error; err == nil {
			detail.Customer = &cust
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ConvDetail{}, err
		}
	}
	var tags []entity.Tag
	if err := r.db.Gorm.WithContext(ctx).
		Joins("JOIN conversation_tags ct ON ct.tag_id = tags.id").
		Where("ct.conversation_id = ?", id).
		Find(&tags).Error; err != nil {
		return ConvDetail{}, err
	}
	detail.Tags = tags
	return detail, nil
}

func (r *readRepo) ListMessages(ctx context.Context, f MsgFilter) ([]entity.Message, error) {
	q := r.db.Gorm.WithContext(ctx).
		Where("conversation_id = ?", f.ConversationID).
		Order("inserted_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if f.Direction == "in" || f.Direction == "out" {
		q = q.Where("direction = ?", f.Direction)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(inserted_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Message
	err := q.Find(&out).Error
	return out, err
}
