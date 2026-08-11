package handlers

type Response struct {
	Status  string
	Message string
}

type FeedResponse struct {
	Articles    []Article `json:"articles"`
	TotalPages  int       `json:"totalPages"`
	TotalItems  int       `json:"totalItems"`
	CurrentPage int       `json:"currentPage"`
	AllTags     []string  `json:"allTags"`
	SelectedTag []string  `json:"selectedTag"`
}

type Article struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}
