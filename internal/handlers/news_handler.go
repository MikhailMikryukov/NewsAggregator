package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	StatusError   = "error"
	StatusSuccess = "success"
)

var ErrNumParameter = errors.New("number must be greater than 0")

type NewsService interface {
	GetCountByTag(ctx context.Context, tags []string) (int, error)
	GetArticlesByTag(ctx context.Context, tags []string, offset int, limit int) ([]Article, error)
	GetAllTags(ctx context.Context) ([]string, error)
}

type RssService interface {
	SaveRss(ctx context.Context, url string) error
}

type NewsHandler struct {
	s  NewsService
	rs RssService
}

func NewNewsHandler(s NewsService, rs RssService) *NewsHandler {
	return &NewsHandler{
		s:  s,
		rs: rs,
	}
}

func NewRouter(s NewsService, rs RssService) *http.ServeMux {
	handler := NewNewsHandler(s, rs)

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "web/static/index.html")
	})

	mux.HandleFunc("/feed", handler.handleFeed)
	mux.HandleFunc("/add_rss", handler.saveSource)

	return mux
}

func (h *NewsHandler) handleFeed(w http.ResponseWriter, r *http.Request) {
	var tags []string

	tagStr := r.FormValue("tag")

	if tagStr != "" {
		tags = strings.Split(tagStr, ",")
	}

	pageStr := r.FormValue("page")
	page, err := validateNumParam(pageStr)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Status:  StatusError,
			Message: "invalid page num: " + err.Error(),
		})
		return
	}

	limitStr := r.FormValue("limit")
	limit, err := validateNumParam(limitStr)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Status:  StatusError,
			Message: "invalid limit num: " + err.Error(),
		})
		return
	}

	ctx := r.Context()

	allArticlesCount, err := h.s.GetCountByTag(ctx, tags)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "error getting tags count",
		})
		return
	}

	if allArticlesCount < page*limit {
		page = allArticlesCount / limit
	}

	offset := page * limit

	articles, err := h.s.GetArticlesByTag(ctx, tags, offset, limit)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "error getting articles by tag",
		})
		return
	}

	allTags, err := h.s.GetAllTags(ctx)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "error getting all tags",
		})
		return
	}

	result := FeedResponse{
		Articles:    articles,
		TotalPages:  allArticlesCount / limit,
		TotalItems:  len(articles),
		CurrentPage: page,
		AllTags:     allTags,
		SelectedTag: tags,
	}

	h.writeJSON(w, http.StatusOK, result)
}

func (h *NewsHandler) saveSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	url := r.FormValue("url")

	err := h.rs.SaveRss(ctx, url)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "error saving url",
		})
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Status:  StatusSuccess,
		Message: "url saved",
	})
}

func (h *NewsHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println(err)
	}
}

func validateNumParam(numStr string) (int, error) {
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return -1, err
	}

	if num < 1 {
		return -1, fmt.Errorf("%w", ErrNumParameter)
	}

	return num, nil
}
