-- +goose Up
CREATE TABLE sources
(
    id      SERIAL PRIMARY KEY,
    rss_url TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE sources;