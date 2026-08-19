package workers

import (
	"context"
	"log"
	"sync"

	"github.com/MikhailMikryukov/NewsAggregator/internal/parser"
)

type Pool struct {
	jobs    chan *Job
	results chan *JobResult
	workers []*RssWorker
}

type Job struct {
	SourceURL string
	SourceId  int
}

type JobResult struct {
	Job  *Job
	Feed *parser.RSSFeed
	Err  error
}

func New(workersNum int, parser *parser.RSSParser) *Pool {
	pool := &Pool{
		workers: make([]*RssWorker, workersNum),
		jobs:    make(chan *Job, workersNum*2),
		results: make(chan *JobResult, workersNum*2),
	}

	for i := range workersNum {
		pool.workers[i] = newWorker(i, parser)
	}
	return pool
}

func (p *Pool) Start(ctx context.Context) {
	var wg sync.WaitGroup

	for i := range p.workers {
		wg.Add(1)
		go func(w *RssWorker) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-p.jobs:
					if !ok {
						return
					}

					feed, err := w.Process(ctx, job.SourceURL)

					p.results <- &JobResult{
						Job:  job,
						Feed: feed,
						Err:  err,
					}

					if err != nil {
						log.Println(err)
						return
					}
				}
			}
		}(p.workers[i])
	}

	go func() {
		wg.Wait()
		close(p.results)
	}()
}

func (p *Pool) Submit(sourceId int, sourceURL string) {
	p.jobs <- &Job{
		SourceId:  sourceId,
		SourceURL: sourceURL,
	}
}

func (p *Pool) Results() <-chan *JobResult {
	return p.results
}
