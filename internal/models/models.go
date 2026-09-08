package models

import "time"

type Source struct {
	RssURL string
	ID     int
}

type Article struct {
	OriginalURL string
	Title       string
	Content     string
	Status      string
	Tags        []string
	ID          int64
	SourceID    int
	PubDate     *time.Time
}
