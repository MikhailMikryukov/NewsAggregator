package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockNewsService struct {
	countByTag    func(ctx context.Context, tags []string) (int, error)
	articlesByTag func(ctx context.Context, tags []string, offset int, limit int) ([]Article, error)
	allTags       func(ctx context.Context) ([]string, error)
}

func (m *mockNewsService) GetCountByTag(ctx context.Context, tags []string) (int, error) {
	return m.countByTag(ctx, tags)
}

func (m *mockNewsService) GetArticlesByTag(ctx context.Context, tags []string, offset int, limit int) ([]Article, error) {
	return m.articlesByTag(ctx, tags, offset, limit)
}

func (m *mockNewsService) GetAllTags(ctx context.Context) ([]string, error) {
	return m.allTags(ctx)
}

type mockRssService struct {
	saveRss func(ctx context.Context, url string) error
}

func (m *mockRssService) SaveRss(ctx context.Context, url string) error {
	return m.saveRss(ctx, url)
}

func TestValidateNumParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "positive number", input: "10", want: 10},
		{name: "minimal number", input: "1", want: 1},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative number", input: "-5", wantErr: true},
		{name: "not a number", input: "abc", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateNumParam(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateNumParam(%q) expected error, got nil", tt.input)
				}
				if tt.input == "0" || tt.input == "-5" {
					if !errors.Is(err, ErrNumParameter) {
						t.Fatalf("validateNumParam(%q) error = %v, want %v", tt.input, err, ErrNumParameter)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("validateNumParam(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("validateNumParam(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandleFeed(t *testing.T) {
	articleDate := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name             string
		url              string
		news             *mockNewsService
		wantStatus       int
		wantJSONKey      string
		wantFeedResponse *FeedResponse
		wantErrorMessage string
		wantOffset       int
		wantLimit        int
	}{
		{
			name: "success with tag",
			url:  "/feed?tag=sport,politics&page=1&limit=10",
			news: &mockNewsService{
				countByTag: func(_ context.Context, tags []string) (int, error) {
					if len(tags) != 2 || tags[0] != "sport" || tags[1] != "politics" {
						t.Errorf("GetCountByTag tags = %v, want [sport politics]", tags)
					}
					return 25, nil
				},
				articlesByTag: func(_ context.Context, tags []string, offset, limit int) ([]Article, error) {
					if len(tags) != 2 {
						t.Errorf("GetArticlesByTag tags = %v, want 2 tags", tags)
					}
					if offset != 10 || limit != 10 {
						t.Errorf("GetArticlesByTag offset/limit = %d/%d, want 10/10", offset, limit)
					}
					return []Article{
						{Title: "a", Content: "content a", Tags: []string{"sport"}, Date: &articleDate},
						{Title: "b", Content: "content b", Tags: []string{"politics"}},
					}, nil
				},
				allTags: func(_ context.Context) ([]string, error) {
					return []string{"sport", "politics"}, nil
				},
			},
			wantStatus:  http.StatusOK,
			wantJSONKey: "articles",
			wantOffset:  10,
			wantLimit:   10,
			wantFeedResponse: &FeedResponse{
				Articles: []Article{
					{Title: "a", Content: "content a", Tags: []string{"sport"}, Date: &articleDate},
					{Title: "b", Content: "content b", Tags: []string{"politics"}},
				},
				AllTags:     []string{"sport", "politics"},
				SelectedTag: []string{"sport", "politics"},
				TotalPages:  2,
				TotalItems:  2,
				CurrentPage: 1,
			},
		},
		{
			name: "success without tag clamps page beyond bounds",
			url:  "/feed?page=3&limit=10",
			news: &mockNewsService{
				countByTag: func(_ context.Context, tags []string) (int, error) {
					return 5, nil
				},
				articlesByTag: func(_ context.Context, _ []string, offset, limit int) ([]Article, error) {
					if offset != 0 {
						t.Errorf("GetArticlesByTag offset = %d, want 0 (page clamped)", offset)
					}
					return nil, nil
				},
				allTags: func(_ context.Context) ([]string, error) {
					return nil, nil
				},
			},
			wantStatus:  http.StatusOK,
			wantJSONKey: "articles",
			wantOffset:  0,
			wantLimit:   10,
			wantFeedResponse: &FeedResponse{
				Articles:    []Article{},
				TotalPages:  0,
				TotalItems:  0,
				CurrentPage: 0,
			},
		},
		{
			name: "invalid page",
			url:  "/feed?page=abc&limit=10",
			news: &mockNewsService{
				countByTag:    func(_ context.Context, _ []string) (int, error) { return 0, nil },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) { return nil, nil },
				allTags:       func(_ context.Context) ([]string, error) { return nil, nil },
			},
			wantStatus:       http.StatusBadRequest,
			wantJSONKey:      "message",
			wantErrorMessage: "invalid page num",
		},
		{
			name: "invalid limit",
			url:  "/feed?page=1&limit=0",
			news: &mockNewsService{
				countByTag:    func(_ context.Context, _ []string) (int, error) { return 0, nil },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) { return nil, nil },
				allTags:       func(_ context.Context) ([]string, error) { return nil, nil },
			},
			wantStatus:       http.StatusBadRequest,
			wantJSONKey:      "message",
			wantErrorMessage: "invalid limit num",
		},
		{
			name: "count error",
			url:  "/feed?page=1&limit=10",
			news: &mockNewsService{
				countByTag:    func(_ context.Context, _ []string) (int, error) { return 0, errors.New("db down") },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) { return nil, nil },
				allTags:       func(_ context.Context) ([]string, error) { return nil, nil },
			},
			wantStatus:       http.StatusInternalServerError,
			wantJSONKey:      "message",
			wantErrorMessage: "error getting tags count",
		},
		{
			name: "articles error",
			url:  "/feed?page=1&limit=10",
			news: &mockNewsService{
				countByTag: func(_ context.Context, _ []string) (int, error) { return 10, nil },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) {
					return nil, errors.New("db down")
				},
				allTags: func(_ context.Context) ([]string, error) { return nil, nil },
			},
			wantStatus:       http.StatusInternalServerError,
			wantJSONKey:      "message",
			wantErrorMessage: "error getting articles by tag",
		},
		{
			name: "all tags error",
			url:  "/feed?page=1&limit=10",
			news: &mockNewsService{
				countByTag:    func(_ context.Context, _ []string) (int, error) { return 10, nil },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) { return nil, nil },
				allTags:       func(_ context.Context) ([]string, error) { return nil, errors.New("db down") },
			},
			wantStatus:       http.StatusInternalServerError,
			wantJSONKey:      "message",
			wantErrorMessage: "error getting all tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewNewsHandler(tt.news, &mockRssService{saveRss: func(_ context.Context, _ string) error { return nil }})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			h.handleFeed(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("handleFeed() status = %d, want %d. Body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantJSONKey == "message" {
				var resp Response
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Status != StatusError {
					t.Errorf("response status = %q, want %q", resp.Status, StatusError)
				}
				if !strings.HasPrefix(resp.Message, tt.wantErrorMessage) {
					t.Errorf("response message = %q, want prefix %q", resp.Message, tt.wantErrorMessage)
				}
				return
			}

			var resp FeedResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode feed response: %v", err)
			}

			if resp.TotalPages != tt.wantFeedResponse.TotalPages {
				t.Errorf("TotalPages = %d, want %d", resp.TotalPages, tt.wantFeedResponse.TotalPages)
			}
			if resp.TotalItems != tt.wantFeedResponse.TotalItems {
				t.Errorf("TotalItems = %d, want %d", resp.TotalItems, tt.wantFeedResponse.TotalItems)
			}
			if resp.CurrentPage != tt.wantFeedResponse.CurrentPage {
				t.Errorf("CurrentPage = %d, want %d", resp.CurrentPage, tt.wantFeedResponse.CurrentPage)
			}
			if len(resp.Articles) != len(tt.wantFeedResponse.Articles) {
				t.Errorf("len(Articles) = %d, want %d", len(resp.Articles), len(tt.wantFeedResponse.Articles))
			}
		})
	}
}

func TestSaveSource(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		saveRss      func(ctx context.Context, url string) error
		wantStatus   int
		wantResponse Response
	}{
		{
			name: "success",
			url:  "/add_rss?url=https://example.com/rss",
			saveRss: func(_ context.Context, recvURL string) error {
				if recvURL != "https://example.com/rss" {
					t.Errorf("SaveRss url = %q, want %q", recvURL, "https://example.com/rss")
				}
				return nil
			},
			wantStatus: http.StatusOK,
			wantResponse: Response{
				Status:  StatusSuccess,
				Message: "url saved",
			},
		},
		{
			name: "repository error",
			url:  "/add_rss?url=https://example.com/rss",
			saveRss: func(_ context.Context, _ string) error {
				return errors.New("db down")
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: Response{
				Status:  StatusError,
				Message: "error saving url",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &mockRssService{saveRss: tt.saveRss}
			h := NewNewsHandler(&mockNewsService{
				countByTag:    func(_ context.Context, _ []string) (int, error) { return 0, nil },
				articlesByTag: func(_ context.Context, _ []string, _, _ int) ([]Article, error) { return nil, nil },
				allTags:       func(_ context.Context) ([]string, error) { return nil, nil },
			}, rs)

			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			w := httptest.NewRecorder()

			h.saveSource(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("saveSource() status = %d, want %d. Body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Status != tt.wantResponse.Status || resp.Message != tt.wantResponse.Message {
				t.Errorf("saveSource() response = %+v, want %+v", resp, tt.wantResponse)
			}
		})
	}
}
