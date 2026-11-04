package repository

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm/clause"
)

var stgColumns = []string{
	"page_id", "conversation_id", "pancake_message_id", "sender_id",
	"direction", "type", "content", "attachments", "inserted_at", "raw",
}

const createStg = `CREATE TEMP TABLE _stg_messages (
	page_id bigint, conversation_id bigint, pancake_message_id text, sender_id text,
	direction text, type text, content text, attachments jsonb, inserted_at timestamptz, raw jsonb
) ON COMMIT DROP`

const insertFromStg = `INSERT INTO messages
	(page_id, conversation_id, pancake_message_id, sender_id, direction, type, content, attachments, inserted_at, raw)
SELECT DISTINCT ON (pancake_message_id, inserted_at)
	page_id, conversation_id, pancake_message_id, sender_id, direction, type, content, attachments, inserted_at, raw
FROM _stg_messages
ORDER BY pancake_message_id, inserted_at
ON CONFLICT (pancake_message_id, inserted_at) DO NOTHING`

type MessageRepository interface {
	UpsertBatch(ctx context.Context, msgs []entity.Message) error
	BulkLoad(ctx context.Context, msgs []entity.Message) (int64, error)
}

type messageRepo struct{ db *DB }

func NewMessageRepository(db *DB) MessageRepository { return &messageRepo{db} }

func (r *messageRepo) ensurePartitions(ctx context.Context, msgs []entity.Message) error {
	for _, m := range msgs {
		if err := r.db.EnsurePartition(ctx, m.InsertedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *messageRepo) UpsertBatch(ctx context.Context, msgs []entity.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := r.ensurePartitions(ctx, msgs); err != nil {
		return err
	}
	// Ensure CreatedAt is set for new records
	for i := range msgs {
		if msgs[i].CreatedAt.IsZero() {
			msgs[i].CreatedAt = msgs[i].InsertedAt
		}
	}
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pancake_message_id"}, {Name: "inserted_at"}},
		DoNothing: true,
	}).CreateInBatches(msgs, 500).Error
}

func (r *messageRepo) BulkLoad(ctx context.Context, msgs []entity.Message) (int64, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	if err := r.ensurePartitions(ctx, msgs); err != nil {
		return 0, err
	}
	conn, err := r.db.Pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, createStg); err != nil {
		return 0, err
	}
	rows := make([][]any, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, []any{
			m.PageID, m.ConversationID, m.PancakeMessageID, m.SenderID,
			m.Direction, m.Type, m.Content, jsonOrNil(m.Attachments), m.InsertedAt, jsonOrNil(m.Raw),
		})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"_stg_messages"}, stgColumns, pgx.CopyFromRows(rows)); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, insertFromStg)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func jsonOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
