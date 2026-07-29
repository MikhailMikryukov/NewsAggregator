package workers

import (
	"NewsAggregator/internal/parser"
	"context"
)

type RssWorker struct {
	id     int
	parser *parser.RSSParser
}

func newWorker(id int, parser *parser.RSSParser) *RssWorker {
	return &RssWorker{
		id:     id,
		parser: parser,
	}
}

func (w *RssWorker) Process(ctx context.Context, url string) (*parser.RSSFeed, error) {
	return w.parser.Parse(ctx, url)
}
