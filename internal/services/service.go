package services

import (
	"NewsAggregator/internal/models"
	"NewsAggregator/internal/rabbitmq"
	"NewsAggregator/internal/repository"
	"NewsAggregator/internal/workers"
	"context"
	"crypto/md5"
	"log"
	"strconv"
)

type Service struct {
	repo      repository.ISourceArticleRepository
	pool      *workers.Pool
	publisher *rabbitmq.Publisher
}

func New(repo repository.ISourceArticleRepository, pool *workers.Pool, publisher *rabbitmq.Publisher) *Service {
	return &Service{
		repo:      repo,
		pool:      pool,
		publisher: publisher,
	}
}

func (s *Service) SetRssJobs(ctx context.Context) {
	rssSources, err := s.repo.GetSources(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	for _, source := range rssSources {
		s.pool.Submit(source.ID, source.RssURL)
	}

}

func (s *Service) HandleJobResult(ctx context.Context, res *workers.JobResult) {
	for _, item := range res.Feed.Channel.Items {
		hash := md5.Sum([]byte(item.Link))

		exists, err := s.repo.CheckArticleByHash(ctx, hash)
		if err != nil {
			log.Println(err)
			return
		}

		if !exists {
			article := models.Article{
				SourceID:    res.Job.SourceId,
				OriginalURL: item.Link,
				Content:     item.Description,
				Tags:        nil,
				Hash:        hash,
				Status:      "pending",
			}

			id, err := s.repo.SaveArticle(ctx, article)
			if err != nil {
				log.Println(err)
				return
			}

			err = s.publisher.Publish("news", []byte(strconv.FormatInt(id, 10)))
			if err != nil {
				log.Println(err)
				return
			}
		}
	}

}
