package workers

import (
	"context"

	"github.com/MikhailMikryukov/NewsAggregator/internal/parser"
)

type RssWorker struct {
	parser *parser.RSSParser
	id     int
}

func newWorker(id int, p *parser.RSSParser) *RssWorker {
	return &RssWorker{
		id:     id,
		parser: p,
	}
}

func (w *RssWorker) Process(ctx context.Context, url string) (*parser.RSSFeed, error) {
	return w.parser.Parse(ctx, url)
}
