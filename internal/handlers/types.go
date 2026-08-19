package handlers

type Response struct {
	Status  string
	Message string
}

type FeedResponse struct {
	Articles    []Article `json:"articles"`
	AllTags     []string  `json:"allTags"`
	SelectedTag []string  `json:"selectedTag"`
	TotalPages  int       `json:"totalPages"`
	TotalItems  int       `json:"totalItems"`
	CurrentPage int       `json:"currentPage"`
}

type Article struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}
