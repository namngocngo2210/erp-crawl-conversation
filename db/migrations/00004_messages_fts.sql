-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages ADD COLUMN IF NOT EXISTS content_tsv tsvector;

CREATE OR REPLACE FUNCTION messages_tsv_update() RETURNS trigger AS $$
BEGIN
  NEW.content_tsv := to_tsvector('simple', coalesce(NEW.content, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_messages_tsv ON messages;
CREATE TRIGGER trg_messages_tsv BEFORE INSERT OR UPDATE OF content ON messages
  FOR EACH ROW EXECUTE FUNCTION messages_tsv_update();

UPDATE messages SET content_tsv = to_tsvector('simple', coalesce(content, ''))
  WHERE content_tsv IS NULL;

CREATE INDEX IF NOT EXISTS idx_messages_content_tsv ON messages USING GIN (content_tsv);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_content_tsv;
DROP TRIGGER IF EXISTS trg_messages_tsv ON messages;
DROP FUNCTION IF EXISTS messages_tsv_update();
ALTER TABLE messages DROP COLUMN IF EXISTS content_tsv;
