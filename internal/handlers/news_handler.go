package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	StatusError = "error"
)

type NewsService interface {
	GetCountByTag(ctx context.Context, tags []string) (int, error)
	GetArticlesByTag(ctx context.Context, tags []string, offset int, limit int) ([]Article, error)
	GetAllTags(ctx context.Context) ([]string, error)
}

type NewsHandler struct {
	s NewsService
}

func NewNewsHandler(s NewsService) *NewsHandler {
	return &NewsHandler{
		s: s,
	}
}

func NewRouter(s NewsService) *http.ServeMux {
	handler := NewNewsHandler(s)

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

	return mux
}

func (h *NewsHandler) handleFeed(w http.ResponseWriter, r *http.Request) {
	var tags []string

	tagStr := r.FormValue("tag")

	if tagStr != "" {
		tags = strings.Split(tagStr, ",")
	}

	pageStr := r.FormValue("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Status:  StatusError,
			Message: "invalid page num",
		})
		return
	}

	limitStr := r.FormValue("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Status:  StatusError,
			Message: "invalid limit num",
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

func (h *NewsHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println(err)
	}
}
