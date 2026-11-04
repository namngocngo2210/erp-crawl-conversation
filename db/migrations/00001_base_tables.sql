-- +goose Up
CREATE TABLE pages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pancake_page_id TEXT NOT NULL UNIQUE,
    platform        TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active',
    last_synced_at  TIMESTAMPTZ,
    raw             JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id             BIGINT NOT NULL REFERENCES pages(id),
    pancake_customer_id TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    phone               TEXT NOT NULL DEFAULT '',
    email               TEXT NOT NULL DEFAULT '',
    fb_id               TEXT NOT NULL DEFAULT '',
    gender              TEXT NOT NULL DEFAULT '',
    raw                 JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_customer_id)
);

CREATE TABLE tags (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id        BIGINT NOT NULL REFERENCES pages(id),
    pancake_tag_id TEXT NOT NULL,
    name           TEXT NOT NULL DEFAULT '',
    color          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_tag_id)
);

CREATE TABLE conversations (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id                 BIGINT NOT NULL REFERENCES pages(id),
    pancake_conversation_id TEXT NOT NULL,
    customer_id             BIGINT REFERENCES customers(id),
    type                    TEXT NOT NULL DEFAULT '',
    snippet                 TEXT NOT NULL DEFAULT '',
    message_count           INT NOT NULL DEFAULT 0,
    last_message_at         TIMESTAMPTZ NOT NULL DEFAULT 'epoch',
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw                     JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_conversation_id)
);
CREATE INDEX idx_conversations_page_updated ON conversations (page_id, updated_at DESC);

CREATE TABLE conversation_tags (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id),
    tag_id          BIGINT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (conversation_id, tag_id)
);

CREATE TABLE crawl_runs (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    trigger     TEXT NOT NULL DEFAULT 'cron',
    status      TEXT NOT NULL DEFAULT 'running',
    stats       JSONB,
    error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE crawl_checkpoints (
    page_id     BIGINT PRIMARY KEY REFERENCES pages(id),
    job_type    TEXT NOT NULL DEFAULT 'conversations',
    cursor      TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT '',
    last_run_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE crawl_checkpoints;
DROP TABLE crawl_runs;
DROP TABLE conversation_tags;
DROP TABLE conversations;
DROP TABLE tags;
DROP TABLE customers;
DROP TABLE pages;
