package models

type Source struct {
	ID     int
	RssURL string
}

type Article struct {
	ID          int64
	SourceID    int
	OriginalURL string
	Title       string
	Content     string
	Tags        []string
	Hash        [16]byte
	Status      string
}
