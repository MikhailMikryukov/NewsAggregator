package services

import (
	"context"
	"crypto/md5"
	"errors"
	"testing"
	"time"

	"github.com/MikhailMikryukov/NewsAggregator/internal/ai"
	"github.com/MikhailMikryukov/NewsAggregator/internal/models"
	"github.com/MikhailMikryukov/NewsAggregator/internal/parser"
	"github.com/MikhailMikryukov/NewsAggregator/internal/workers"
)

type mockArticleRepo struct {
	saveArticle   func(ctx context.Context, a models.Article, hash [16]byte) (int64, error)
	getArticle    func(ctx context.Context, id int64) (*models.Article, error)
	updateArticle func(ctx context.Context, a models.Article) error
	updateStatus  func(ctx context.Context, articleID int64, status string) error
	countByTag    func(ctx context.Context, tag []string) (int, error)
	articlesByTag func(ctx context.Context, tag []string, offset int, limit int) ([]models.Article, error)
	allTags       func(ctx context.Context) ([]string, error)
}

func (m *mockArticleRepo) SaveArticle(ctx context.Context, a models.Article, hash [16]byte) (int64, error) {
	return m.saveArticle(ctx, a, hash)
}

func (m *mockArticleRepo) GetArticle(ctx context.Context, id int64) (*models.Article, error) {
	return m.getArticle(ctx, id)
}

func (m *mockArticleRepo) UpdateArticle(ctx context.Context, a models.Article) error {
	return m.updateArticle(ctx, a)
}

func (m *mockArticleRepo) UpdateStatus(ctx context.Context, articleID int64, status string) error {
	return m.updateStatus(ctx, articleID, status)
}

func (m *mockArticleRepo) GetCountByTag(ctx context.Context, tag []string) (int, error) {
	return m.countByTag(ctx, tag)
}

func (m *mockArticleRepo) GetArticlesByTag(ctx context.Context, tag []string, offset int, limit int) ([]models.Article, error) {
	return m.articlesByTag(ctx, tag, offset, limit)
}

func (m *mockArticleRepo) GetAllTags(ctx context.Context) ([]string, error) {
	return m.allTags(ctx)
}

type mockSourceRepo struct {
	getSources func(ctx context.Context) ([]models.Source, error)
	saveSource func(ctx context.Context, rssURL string) error
}

func (m *mockSourceRepo) GetSources(ctx context.Context) ([]models.Source, error) {
	return m.getSources(ctx)
}

func (m *mockSourceRepo) SaveSource(ctx context.Context, rssURL string) error {
	return m.saveSource(ctx, rssURL)
}

type mockTagger struct {
	generateTags func(ctx context.Context, req ai.TagRequest) (*ai.TagResponse, error)
	healthCheck  func(ctx context.Context) error
}

func (m *mockTagger) GenerateTags(ctx context.Context, req ai.TagRequest) (*ai.TagResponse, error) {
	return m.generateTags(ctx, req)
}

func (m *mockTagger) HealthCheck(ctx context.Context) error {
	return m.healthCheck(ctx)
}

type mockPublisher struct {
	publish func(routingKey string, body []byte) error
}

func (m *mockPublisher) Publish(routingKey string, body []byte) error {
	return m.publish(routingKey, body)
}

type mockJobSubmitter struct {
	submit func(ctx context.Context, sourceId int, sourceURL string)
}

func (m *mockJobSubmitter) Submit(ctx context.Context, sourceId int, sourceURL string) {
	m.submit(ctx, sourceId, sourceURL)
}

func TestHandleArticle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		articleID      int64
		articleRepo    *mockArticleRepo
		ai             *mockTagger
		wantErr        bool
		wantStatus     string
		wantCallTags   bool
		wantCallUpdate bool
	}{
		{
			name:      "article already completed",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, id int64) (*models.Article, error) {
					return &models.Article{ID: id, Status: "completed"}, nil
				},
				updateStatus:  func(_ context.Context, _ int64, _ string) error { return errors.New("unexpected") },
				updateArticle: func(_ context.Context, _ models.Article) error { return errors.New("unexpected") },
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, _ ai.TagRequest) (*ai.TagResponse, error) {
					return nil, errors.New("unexpected")
				},
			},
			wantErr:      false,
			wantStatus:   "completed",
			wantCallTags: false,
		},
		{
			name:      "get article error",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, _ int64) (*models.Article, error) {
					return nil, errors.New("db down")
				},
				updateStatus:  func(_ context.Context, _ int64, _ string) error { return nil },
				updateArticle: func(_ context.Context, _ models.Article) error { return nil },
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, _ ai.TagRequest) (*ai.TagResponse, error) {
					return nil, errors.New("unexpected")
				},
			},
			wantErr:        true,
			wantCallTags:   false,
			wantCallUpdate: false,
		},
		{
			name:      "update status to processing error",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, id int64) (*models.Article, error) {
					return &models.Article{ID: id, Status: "pending"}, nil
				},
				updateStatus: func(_ context.Context, _ int64, status string) error {
					if status == "processing" {
						return errors.New("db down")
					}
					return nil
				},
				updateArticle: func(_ context.Context, _ models.Article) error { return nil },
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, _ ai.TagRequest) (*ai.TagResponse, error) {
					return nil, errors.New("unexpected")
				},
			},
			wantErr:        true,
			wantCallTags:   false,
			wantCallUpdate: false,
		},
		{
			name:      "ai generate tags error",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, id int64) (*models.Article, error) {
					return &models.Article{ID: id, Status: "pending"}, nil
				},
				updateStatus: func(_ context.Context, _ int64, status string) error {
					if status == "failed" {
						return nil
					}
					return nil
				},
				updateArticle: func(_ context.Context, _ models.Article) error { return nil },
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, _ ai.TagRequest) (*ai.TagResponse, error) {
					return nil, errors.New("ai down")
				},
			},
			wantErr:      true,
			wantStatus:   "failed",
			wantCallTags: true,
		},
		{
			name:      "update article error",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, id int64) (*models.Article, error) {
					return &models.Article{ID: id, Status: "pending"}, nil
				},
				updateStatus: func(_ context.Context, _ int64, status string) error {
					if status == "failed" {
						return nil
					}
					return nil
				},
				updateArticle: func(_ context.Context, _ models.Article) error {
					return errors.New("db down")
				},
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, _ ai.TagRequest) (*ai.TagResponse, error) {
					return &ai.TagResponse{Tags: []string{"test"}}, nil
				},
			},
			wantErr:        true,
			wantCallUpdate: true,
		},
		{
			name:      "success",
			articleID: 1,
			articleRepo: &mockArticleRepo{
				getArticle: func(_ context.Context, id int64) (*models.Article, error) {
					return &models.Article{ID: id, Title: "news", Content: "long description", Status: "pending"}, nil
				},
				updateStatus: func(_ context.Context, _ int64, _ string) error {
					return nil
				},
				updateArticle: func(_ context.Context, a models.Article) error {
					if a.Status != "completed" {
						t.Errorf("UpdateArticle status = %q, want completed", a.Status)
					}
					if len(a.Tags) != 1 || a.Tags[0] != "tech" {
						t.Errorf("UpdateArticle tags = %v, want [tech]", a.Tags)
					}
					return nil
				},
			},
			ai: &mockTagger{
				generateTags: func(_ context.Context, req ai.TagRequest) (*ai.TagResponse, error) {
					if req.Description == "" {
						t.Error("GenerateTags received empty description")
					}
					return &ai.TagResponse{Tags: []string{"tech"}}, nil
				},
			},
			wantErr:        false,
			wantStatus:     "completed",
			wantCallTags:   true,
			wantCallUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(tt.articleRepo, nil, nil, nil, tt.ai)
			err := svc.HandleArticle(context.Background(), tt.articleID)

			if tt.wantErr && err == nil {
				t.Errorf("HandleArticle() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("HandleArticle() unexpected error: %v", err)
			}
		})
	}
}

func TestHandleJobResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		res         *workers.JobResult
		articleRepo *mockArticleRepo
		publisher   *mockPublisher
	}{
		{
			name: "result has error",
			res: &workers.JobResult{
				Job:  &workers.Job{SourceId: 1},
				Feed: nil,
				Err:  errors.New("parsing failed"),
			},
			articleRepo: &mockArticleRepo{
				saveArticle: func(_ context.Context, _ models.Article, _ [16]byte) (int64, error) {
					return 0, errors.New("unexpected")
				},
			},
			publisher: &mockPublisher{
				publish: func(_ string, _ []byte) error {
					return errors.New("unexpected")
				},
			},
		},
		{
			name: "duplicate article - save returns conflict id -1",
			res: &workers.JobResult{
				Job: &workers.Job{SourceId: 1},
				Feed: &parser.RSSFeed{
					Channel: parser.Channel{
						Items: []parser.Item{
							{Title: "dup", Link: "https://example.com/dup"},
						},
					},
				},
			},
			articleRepo: &mockArticleRepo{
				saveArticle: func(_ context.Context, a models.Article, hash [16]byte) (int64, error) {
					expectedHash := md5.Sum([]byte("https://example.com/dup"))
					if hash != expectedHash {
						t.Errorf("SaveArticle hash = %x, want %x", hash, expectedHash)
					}
					return -1, nil
				},
				updateStatus: func(_ context.Context, _ int64, _ string) error {
					return errors.New("unexpected")
				},
			},
			publisher: &mockPublisher{
				publish: func(_ string, _ []byte) error {
					return errors.New("unexpected")
				},
			},
		},
		{
			name: "save article error",
			res: &workers.JobResult{
				Job: &workers.Job{SourceId: 1},
				Feed: &parser.RSSFeed{
					Channel: parser.Channel{
						Items: []parser.Item{
							{Title: "a", Link: "https://example.com/a"},
						},
					},
				},
			},
			articleRepo: &mockArticleRepo{
				saveArticle: func(_ context.Context, _ models.Article, _ [16]byte) (int64, error) {
					return 0, errors.New("db down")
				},
				updateStatus: func(_ context.Context, _ int64, _ string) error {
					return errors.New("unexpected")
				},
			},
			publisher: &mockPublisher{
				publish: func(_ string, _ []byte) error {
					return errors.New("unexpected")
				},
			},
		},
		{
			name: "publish error",
			res: &workers.JobResult{
				Job: &workers.Job{SourceId: 1},
				Feed: &parser.RSSFeed{
					Channel: parser.Channel{
						Items: []parser.Item{
							{Title: "a", Link: "https://example.com/a"},
						},
					},
				},
			},
			articleRepo: &mockArticleRepo{
				saveArticle: func(_ context.Context, _ models.Article, _ [16]byte) (int64, error) {
					return 42, nil
				},
				updateStatus: func(_ context.Context, _ int64, _ string) error {
					return errors.New("unexpected")
				},
			},
			publisher: &mockPublisher{
				publish: func(_ string, body []byte) error {
					if string(body) != "42" {
						t.Errorf("Publish body = %q, want '42'", string(body))
					}
					return errors.New("rabbit down")
				},
			},
		},
		{
			name: "success",
			res: &workers.JobResult{
				Job: &workers.Job{SourceId: 1},
				Feed: &parser.RSSFeed{
					Channel: parser.Channel{
						Items: []parser.Item{
							{Title: "a", Link: "https://example.com/a"},
							{Title: "b", Link: "https://example.com/b"},
						},
					},
				},
			},
			articleRepo: &mockArticleRepo{
				saveArticle: func(_ context.Context, a models.Article, hash [16]byte) (int64, error) {
					if a.Title == "a" {
						return 1, nil
					}
					return 2, nil
				},
				updateStatus: func(_ context.Context, id int64, status string) error {
					if status != "queued" {
						t.Errorf("UpdateStatus status = %q, want queued", status)
					}
					return nil
				},
			},
			publisher: &mockPublisher{
				publish: func(key string, body []byte) error {
					if key != "news" {
						t.Errorf("Publish routingKey = %q, want 'news'", key)
					}
					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(tt.articleRepo, nil, nil, tt.publisher, nil)
			svc.HandleJobResult(context.Background(), tt.res)
		})
	}
}

func TestGetArticlesByTag(t *testing.T) {
	t.Parallel()

	now := nowPtr()

	tests := []struct {
		name       string
		tags       []string
		offset     int
		limit      int
		repo       *mockArticleRepo
		wantCount  int
		wantErr    bool
		wantTitles []string
	}{
		{
			name:   "repo error",
			tags:   []string{"tech"},
			offset: 0,
			limit:  10,
			repo: &mockArticleRepo{
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]models.Article, error) {
					return nil, errors.New("db down")
				},
			},
			wantErr: true,
		},
		{
			name:   "success with date mapping",
			tags:   []string{"tech"},
			offset: 0,
			limit:  10,
			repo: &mockArticleRepo{
				articlesByTag: func(_ context.Context, tags []string, _, _ int) ([]models.Article, error) {
					if len(tags) != 1 || tags[0] != "tech" {
						t.Errorf("GetArticlesByTag tags = %v, want [tech]", tags)
					}
					return []models.Article{
						{Title: "a", Content: "content a", Tags: []string{"tech"}, PubDate: now},
						{Title: "b", Content: "content b", Tags: []string{"tech"}},
					}, nil
				},
			},
			wantCount:  2,
			wantTitles: []string{"a", "b"},
		},
		{
			name:   "empty result",
			tags:   nil,
			offset: 0,
			limit:  10,
			repo: &mockArticleRepo{
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]models.Article, error) {
					return nil, nil
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(tt.repo, nil, nil, nil, nil)
			got, err := svc.GetArticlesByTag(context.Background(), tt.tags, tt.offset, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetArticlesByTag() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetArticlesByTag() unexpected error: %v", err)
			}

			if len(got) != tt.wantCount {
				t.Fatalf("GetArticlesByTag() count = %d, want %d", len(got), tt.wantCount)
			}

			for i, a := range got {
				if tt.wantTitles != nil && a.Title != tt.wantTitles[i] {
					t.Errorf("GetArticlesByTag()[%d].Title = %q, want %q", i, a.Title, tt.wantTitles[i])
				}
				if a.Tags == nil {
					a.Tags = []string{}
				}
			}
		})
	}
}

func TestSetRssJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceRepo *mockSourceRepo
		pool       *mockJobSubmitter
		wantSubmit []int
	}{
		{
			name: "source repo error",
			sourceRepo: &mockSourceRepo{
				getSources: func(_ context.Context) ([]models.Source, error) {
					return nil, errors.New("db down")
				},
			},
			pool: &mockJobSubmitter{
				submit: func(_ context.Context, _ int, _ string) {
					t.Error("Submit should not be called on error")
				},
			},
		},
		{
			name: "success submits all sources",
			sourceRepo: &mockSourceRepo{
				getSources: func(_ context.Context) ([]models.Source, error) {
					return []models.Source{
						{ID: 1, RssURL: "https://example.com/1"},
						{ID: 2, RssURL: "https://example.com/2"},
					}, nil
				},
			},
			pool: &mockJobSubmitter{
				submit: func(_ context.Context, id int, url string) {
				},
			},
		},
		{
			name: "empty sources",
			sourceRepo: &mockSourceRepo{
				getSources: func(_ context.Context) ([]models.Source, error) {
					return []models.Source{}, nil
				},
			},
			pool: &mockJobSubmitter{
				submit: func(_ context.Context, _ int, _ string) {
					t.Error("Submit should not be called on empty sources")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(nil, tt.sourceRepo, tt.pool, nil, nil)
			svc.SetRssJobs(context.Background())
		})
	}
}

func TestSaveRss(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		repo    *mockSourceRepo
		wantErr bool
	}{
		{
			name: "success",
			url:  "https://example.com/rss",
			repo: &mockSourceRepo{
				saveSource: func(_ context.Context, url string) error {
					if url != "https://example.com/rss" {
						t.Errorf("SaveSource url = %q, want %q", url, "https://example.com/rss")
					}
					return nil
				},
			},
		},
		{
			name: "repo error",
			url:  "https://example.com/rss",
			repo: &mockSourceRepo{
				saveSource: func(_ context.Context, _ string) error {
					return errors.New("db down")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := New(nil, tt.repo, nil, nil, nil)
			err := svc.SaveRss(context.Background(), tt.url)

			if tt.wantErr && err == nil {
				t.Errorf("SaveRss() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("SaveRss() unexpected error: %v", err)
			}
		})
	}
}

func nowPtr() *time.Time {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &t
}
