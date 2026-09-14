package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"strconv"

	"github.com/MikhailMikryukov/NewsAggregator/internal/ai"
	"github.com/MikhailMikryukov/NewsAggregator/internal/handlers"
	"github.com/MikhailMikryukov/NewsAggregator/internal/models"
	"github.com/MikhailMikryukov/NewsAggregator/internal/rabbitmq"
	"github.com/MikhailMikryukov/NewsAggregator/internal/repository"
	"github.com/MikhailMikryukov/NewsAggregator/internal/workers"
)

type Service struct {
	articleRepo repository.ArticleRepository
	sourceRepo  repository.SourceRepository
	pool        *workers.Pool
	publisher   *rabbitmq.Publisher
	ai          ai.Tagger
}

func New(ar repository.ArticleRepository, sr repository.SourceRepository, pool *workers.Pool, publisher *rabbitmq.Publisher, ai ai.Tagger) *Service {
	return &Service{
		articleRepo: ar,
		sourceRepo:  sr,
		pool:        pool,
		publisher:   publisher,
		ai:          ai,
	}
}

func (s *Service) SetRssJobs(ctx context.Context) {
	rssSources, err := s.sourceRepo.GetSources(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	for _, source := range rssSources {
		s.pool.Submit(ctx, source.ID, source.RssURL)
	}
}

func (s *Service) HandleJobResult(ctx context.Context, res *workers.JobResult) {

	if res.Err != nil {
		log.Println(res.Err)
		return
	}

	for _, item := range res.Feed.Channel.Items {
		hash := md5.Sum([]byte(item.Link))

		article := models.Article{
			SourceID:    res.Job.SourceId,
			OriginalURL: item.Link,
			Title:       item.Title,
			Content:     item.Description,
			Tags:        nil,
			Status:      "pending",
		}

		id, err := s.articleRepo.SaveArticle(ctx, article, hash)
		if err != nil {
			log.Printf("failed to save article: %v", err)
			continue
		}

		if id > 0 {
			err = s.publisher.Publish("news", []byte(strconv.FormatInt(id, 10)))
			if err != nil {
				log.Printf("failed to save publish: %v", err)
				continue
			}

			err = s.articleRepo.UpdateStatus(ctx, id, "queued")
			if err != nil {
				log.Printf("failed to update status: %v", err)
				continue
			}
		}
	}
}

func (s *Service) HandleArticle(ctx context.Context, id int64) error {
	article, err := s.articleRepo.GetArticle(ctx, id)
	if err != nil {
		return err
	}

	if article.Status == "completed" {
		log.Printf("article already proceed %d", id)
		return nil
	}

	tagReq := ai.TagRequest{
		Description: article.Content,
		Title:       article.Title,
	}

	err = s.articleRepo.UpdateStatus(ctx, article.ID, "processing")
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	tagResp, err := s.ai.GenerateTags(ctx, tagReq)
	if err != nil {
		err = s.articleRepo.UpdateStatus(ctx, article.ID, "failed")
		if err != nil {
			log.Printf("failed to update status: %v", err)
		}
		return fmt.Errorf("generating tags error: %w", err)
	}

	article.Tags = tagResp.Tags
	article.Status = "completed"

	err = s.articleRepo.UpdateArticle(ctx, *article)
	if err != nil {
		return fmt.Errorf("updating article error: %w", err)
	}

	return nil
}

func (s *Service) GetCountByTag(ctx context.Context, tags []string) (int, error) {
	return s.articleRepo.GetCountByTag(ctx, tags)
}

func (s *Service) GetArticlesByTag(ctx context.Context, tags []string, offset int, limit int) ([]handlers.Article, error) {
	articles, err := s.articleRepo.GetArticlesByTag(ctx, tags, offset, limit)
	if err != nil {
		return nil, err
	}

	result := make([]handlers.Article, len(articles))
	for i := 0; i < len(articles); i++ {
		result[i].Title = articles[i].Title
		result[i].Tags = articles[i].Tags
		result[i].Content = articles[i].Content
		result[i].Date = articles[i].PubDate
	}

	return result, nil
}

func (s *Service) GetAllTags(ctx context.Context) ([]string, error) {
	return s.articleRepo.GetAllTags(ctx)
}

func (s *Service) SaveRss(ctx context.Context, url string) error {
	return s.sourceRepo.SaveSource(ctx, url)
}
