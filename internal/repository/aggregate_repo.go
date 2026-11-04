package repository

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormTx = gorm.DB

type PageRepository interface {
	Upsert(ctx context.Context, p entity.Page) (int64, error)
}
type CustomerRepository interface {
	Upsert(ctx context.Context, c entity.Customer) (int64, error)
}
type TagRepository interface {
	Upsert(ctx context.Context, t entity.Tag) (int64, error)
}
type ConversationRepository interface {
	Upsert(ctx context.Context, c entity.Conversation) (int64, error)
	SetTags(ctx context.Context, conversationID int64, tagIDs []int64) error
}

type pageRepo struct{ db *DB }
type customerRepo struct{ db *DB }
type tagRepo struct{ db *DB }
type conversationRepo struct{ db *DB }

func NewPageRepository(db *DB) PageRepository                 { return &pageRepo{db} }
func NewCustomerRepository(db *DB) CustomerRepository         { return &customerRepo{db} }
func NewTagRepository(db *DB) TagRepository                   { return &tagRepo{db} }
func NewConversationRepository(db *DB) ConversationRepository { return &conversationRepo{db} }

func (r *pageRepo) Upsert(ctx context.Context, p entity.Page) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pancake_page_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "platform", "status", "raw", "updated_at"}),
	}).Create(&p).Error
	return p.ID, err
}

func (r *customerRepo) Upsert(ctx context.Context, c entity.Customer) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_customer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "phone", "email", "fb_id", "gender", "raw", "updated_at"}),
	}).Create(&c).Error
	return c.ID, err
}

func (r *tagRepo) Upsert(ctx context.Context, t entity.Tag) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_tag_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "color", "updated_at"}),
	}).Create(&t).Error
	return t.ID, err
}

func (r *conversationRepo) Upsert(ctx context.Context, c entity.Conversation) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_conversation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"customer_id", "type", "snippet", "message_count", "last_message_at", "updated_at", "raw"}),
	}).Create(&c).Error
	return c.ID, err
}

func (r *conversationRepo) SetTags(ctx context.Context, conversationID int64, tagIDs []int64) error {
	return r.db.Gorm.WithContext(ctx).Transaction(func(tx *gormTx) error {
		if err := tx.Exec("DELETE FROM conversation_tags WHERE conversation_id = ?", conversationID).Error; err != nil {
			return err
		}
		for _, tid := range tagIDs {
			if err := tx.Exec("INSERT INTO conversation_tags (conversation_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING", conversationID, tid).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
