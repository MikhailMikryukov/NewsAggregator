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
	itemsPerPage = 10
	StatusError  = "error"
)

type NewsService interface {
	GetCountByTag(ctx context.Context, tags []string) (int, error)
	GetArticlesByTag(ctx context.Context, tags []string, offset int) ([]Article, error)
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

	mux.HandleFunc("/feed", handler.handleFeed)

	return mux
}

func (h *NewsHandler) handleFeed(w http.ResponseWriter, r *http.Request) {
	tagStr := r.FormValue("tag")
	tags := strings.Split(tagStr, " ")

	pageStr := r.FormValue("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Status:  StatusError,
			Message: "invalid page num",
		})
		return
	}

	ctx := r.Context()

	allArticlesCount, err := h.s.GetCountByTag(ctx, tags)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "",
		})
		return
	}

	if allArticlesCount < page*itemsPerPage {
		page = allArticlesCount / itemsPerPage
	}

	offset := page * itemsPerPage

	articles, err := h.s.GetArticlesByTag(ctx, tags, offset)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "",
		})
		return
	}

	allTags, err := h.s.GetAllTags(ctx)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Status:  StatusError,
			Message: "",
		})
		return
	}

	result := FeedResponse{
		Articles:    articles,
		TotalPages:  allArticlesCount / itemsPerPage,
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
