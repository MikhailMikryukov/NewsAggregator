-- +goose Up
CREATE TABLE articles
(
    id           BIGSERIAL PRIMARY KEY,
    source_id    INTEGER NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    original_url TEXT    NOT NULL UNIQUE,
    title        TEXT    NOT NULL,
    content      TEXT,
    tags         TEXT[],
    pub_date     TIMESTAMPTZ,
    status       TEXT    NOT NULL DEFAULT 'new',
    hash         TEXT    NOT NULL UNIQUE,
);

-- +goose Down
DROP TABLE articles;