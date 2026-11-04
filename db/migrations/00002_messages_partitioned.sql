-- +goose Up
CREATE TABLE messages (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY,
    page_id            BIGINT NOT NULL,
    conversation_id    BIGINT NOT NULL,
    pancake_message_id TEXT NOT NULL,
    sender_id          TEXT NOT NULL DEFAULT '',
    direction          TEXT NOT NULL DEFAULT '',
    type               TEXT NOT NULL DEFAULT '',
    content            TEXT NOT NULL DEFAULT '',
    attachments        JSONB,
    inserted_at        TIMESTAMPTZ NOT NULL,
    raw                JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, inserted_at),
    UNIQUE (pancake_message_id, inserted_at)
) PARTITION BY RANGE (inserted_at);

CREATE INDEX idx_messages_conv_inserted ON messages (conversation_id, inserted_at DESC);

CREATE TABLE messages_default PARTITION OF messages DEFAULT;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_messages_partition(month_start date)
RETURNS void AS $$
DECLARE
    part_name text := 'messages_y' || to_char(month_start, 'YYYY') || 'm' || to_char(month_start, 'MM');
    month_end date := (month_start + INTERVAL '1 month')::date;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF messages FOR VALUES FROM (%L) TO (%L)',
            part_name, month_start, month_end
        );
    END IF;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS ensure_messages_partition(date);
DROP TABLE messages;
