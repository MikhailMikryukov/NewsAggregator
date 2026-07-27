package models

type Source struct {
	ID     int
	RssURL string
}

type Article struct {
	ID          int
	SourceID    int
	OriginalURL string
	Content     string
	Tags        []string
	Hash        [16]byte
	Status      string
}
