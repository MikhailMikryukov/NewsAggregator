package ai

import (
	"errors"
	"strings"
	"testing"
)

func TestParseResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
		wantErr error
	}{
		{
			name:    "clean JSON",
			content: `{"tags": ["tech", "news"]}`,
			want:    []string{"tech", "news"},
		},
		{
			name:    "JSON wrapped in text",
			content: "Here is your answer: {\"tags\": [\"tech\"]} thank you",
			want:    []string{"tech"},
		},
		{
			name:    "JSON with extra whitespace",
			content: "  {\"tags\": [\"tech\", \"politics\", \"sport\"]}  ",
			want:    []string{"tech", "politics", "sport"},
		},
		{
			name:    "no braces",
			content: "tags: tech, news",
			wantErr: ErrJSONNotFound,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: ErrJSONNotFound,
		},
		{
			name:    "stray brace before JSON breaks extraction",
			content: "{prefix {\"tags\": [\"go\"]} suffix",
			wantErr: errors.New("json parsing error"),
		},
		{
			name:    "invalid JSON inside braces",
			content: `{"tags": [unclosed`,
			wantErr: ErrJSONNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &YandexAIClient{}

			got, err := c.parseResponse(tt.content)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("parseResponse() expected error, got tags %v", got)
				}
				if tt.wantErr == ErrJSONNotFound && !errors.Is(err, ErrJSONNotFound) {
					t.Errorf("parseResponse() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseResponse() unexpected error: %v", err)
			}

			if len(got.Tags) != len(tt.want) {
				t.Fatalf("parseResponse() tags = %v, want %v", got.Tags, tt.want)
			}

			for i := range tt.want {
				if got.Tags[i] != tt.want[i] {
					t.Errorf("parseResponse() tags[%d] = %q, want %q", i, got.Tags[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		req        TagRequest
		maxTags    int
		wantHas    []string
		wantNotHas []string
	}{
		{
			name:    "with title and description",
			req:     TagRequest{Title: "Go update", Description: "New release"},
			maxTags: 5,
			wantHas: []string{
				"Максимальное количество тегов: 5",
				"Заголовок статьи: Go update",
				"Описание: New release",
				`{"tags": ["тег1", "тег2", "тег3"]}`,
			},
			wantNotHas: []string{
				"Заголовок статьи:\n",
			},
		},
		{
			name:    "without title",
			req:     TagRequest{Description: "Only description"},
			maxTags: 3,
			wantHas: []string{
				"Максимальное количество тегов: 3",
				"Описание: Only description",
			},
			wantNotHas: []string{
				"Заголовок статьи:",
			},
		},
		{
			name:    "with empty description",
			req:     TagRequest{Description: ""},
			maxTags: 10,
			wantHas: []string{
				"Максимальное количество тегов: 10",
				"Описание: ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &YandexAIClient{}
			prompt := c.buildPrompt(tt.req, tt.maxTags)

			for _, s := range tt.wantHas {
				if !strings.Contains(prompt, s) {
					t.Errorf("buildPrompt() missing %q in:\n%s", s, prompt)
				}
			}

			for _, s := range tt.wantNotHas {
				if strings.Contains(prompt, s) {
					t.Errorf("buildPrompt() should not contain %q in:\n%s", s, prompt)
				}
			}
		})
	}
}
