package workers

import (
	"NewsAggregator/internal/parser"
	"context"
	"sync"
)

type Pool struct {
	workers chan *RssWorker
	jobs    chan string
	results chan *JobResult
}

type JobResult struct {
	URL  string
	Feed *parser.RSSFeed
	Err  error
}

func New(workersNum int, parser *parser.RSSParser) *Pool {
	pool := &Pool{
		workers: make(chan *RssWorker, workersNum),
		jobs:    make(chan string, 20),
		results: make(chan *JobResult, 20),
	}

	for i := range workersNum {
		pool.workers <- newWorker(i, parser)
	}
	return pool
}

func (p *Pool) Start(ctx context.Context) {
	var wg sync.WaitGroup

	for worker := range p.workers {
		wg.Add(1)
		go func(w *RssWorker) {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-p.jobs:
				if !ok {
					return
				}

				feed, err := w.Process(ctx, job)
				if err != nil {
					return
				}

				p.results <- &JobResult{
					URL:  job,
					Feed: feed,
					Err:  err,
				}
			}
		}(worker)
	}

	go func() {
		wg.Wait()
		close(p.results)
	}()
}
