package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseRSSDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dateStr string
		want    time.Time
		wantErr error
	}{
		{
			name:    "RFC1123Z",
			dateStr: "Mon, 02 Jan 2006 15:04:05 -0700",
			want:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*60*60)),
		},
		{
			name:    "RFC1123 without zone offset",
			dateStr: "Mon, 02 Jan 2006 15:04:05 MST",
			want:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name:    "RFC822Z",
			dateStr: "02 Jan 06 15:04 -0700",
			want:    time.Date(2006, 1, 2, 15, 4, 0, 0, time.FixedZone("", -7*60*60)),
		},
		{
			name:    "RFC822 without zone offset",
			dateStr: "02 Jan 06 15:04 MST",
			want:    time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC),
		},
		{
			name:    "ISO8601 UTC",
			dateStr: "2006-01-02T15:04:05Z",
			want:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name:    "ISO8601 with offset",
			dateStr: "2006-01-02T15:04:05-07:00",
			want:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*60*60)),
		},
		{
			name:    "empty string",
			dateStr: "",
			wantErr: ErrParsing,
		},
		{
			name:    "garbage",
			dateStr: "not a date",
			wantErr: ErrParsing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseRSSDate(tt.dateStr)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseRSSDate(%q) error = %v, want %v", tt.dateStr, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseRSSDate(%q) unexpected error: %v", tt.dateStr, err)
			}

			if !got.Equal(tt.want) {
				t.Errorf("parseRSSDate(%q) = %v, want %v", tt.dateStr, got, tt.want)
			}
		})
	}
}

func TestParseDates(t *testing.T) {
	t.Parallel()

	valid := "Mon, 02 Jan 2006 15:04:05 -0700"
	wantDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*60*60))

	tests := []struct {
		name  string
		items []Item
		want  []*time.Time
	}{
		{
			name: "all valid",
			items: []Item{
				{Title: "a", PubDate: valid},
				{Title: "b", PubDate: "2006-01-02T15:04:05Z"},
			},
			want: []*time.Time{
				&wantDate,
				func() *time.Time {
					t := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
					return &t
				}(),
			},
		},
		{
			name: "empty date keeps nil",
			items: []Item{
				{Title: "no date", PubDate: ""},
			},
			want: []*time.Time{nil},
		},
		{
			name: "invalid date keeps nil",
			items: []Item{
				{Title: "bad date", PubDate: "yesterday, maybe"},
			},
			want: []*time.Time{nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			feed := &RSSFeed{Channel: Channel{Items: tt.items}}
			p := &RSSParser{}

			p.parseDates(feed)

			for i, item := range feed.Channel.Items {
				if tt.want[i] == nil {
					if item.ParsedPubDate != nil {
						t.Errorf("item[%d] ParsedPubDate = %v, want nil", i, item.ParsedPubDate)
					}
					continue
				}
				if item.ParsedPubDate == nil {
					t.Errorf("item[%d] ParsedPubDate = nil, want %v", i, *tt.want[i])
					continue
				}
				if !item.ParsedPubDate.Equal(*tt.want[i]) {
					t.Errorf("item[%d] ParsedPubDate = %v, want %v", i, *item.ParsedPubDate, *tt.want[i])
				}
			}
		})
	}
}

func TestParseFeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want RSSFeed
	}{
		{
			name: "valid feed",
			body: `<rss><channel><title>News</title>` +
				`<item><title>First</title><link>https://example.com/1</link></item>` +
				`<item><title>Second</title><link>https://example.com/2</link></item>` +
				`</channel></rss>`,
			want: RSSFeed{
				Channel: Channel{
					Title: "News",
					Items: []Item{
						{Title: "First", Link: "https://example.com/1"},
						{Title: "Second", Link: "https://example.com/2"},
					},
				},
			},
		},
		{
			name: "minimal feed",
			body: `<rss><channel></channel></rss>`,
			want: RSSFeed{Channel: Channel{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &RSSParser{}

			got, err := p.parseFeed([]byte(tt.body))
			if err != nil {
				t.Fatalf("parseFeed() unexpected error: %v", err)
			}

			if len(got.Channel.Items) != len(tt.want.Channel.Items) {
				t.Fatalf("parseFeed() items count = %d, want %d", len(got.Channel.Items), len(tt.want.Channel.Items))
			}

			if got.Channel.Title != tt.want.Channel.Title {
				t.Errorf("parseFeed() title = %q, want %q", got.Channel.Title, tt.want.Channel.Title)
			}

			for i := range tt.want.Channel.Items {
				if got.Channel.Items[i].Title != tt.want.Channel.Items[i].Title {
					t.Errorf("parseFeed() items[%d].Title = %q, want %q",
						i, got.Channel.Items[i].Title, tt.want.Channel.Items[i].Title)
				}
				if got.Channel.Items[i].Link != tt.want.Channel.Items[i].Link {
					t.Errorf("parseFeed() items[%d].Link = %q, want %q",
						i, got.Channel.Items[i].Link, tt.want.Channel.Items[i].Link)
				}
			}
		})
	}
}

func TestParseFeedInvalidXML(t *testing.T) {
	t.Parallel()

	p := &RSSParser{}

	_, err := p.parseFeed([]byte(`<rss><channel>`))
	if err == nil {
		t.Fatal("parseFeed() expected error for malformed XML, got nil")
	}
}

func TestReadBody(t *testing.T) {
	t.Parallel()

	const maxSize = 10 * 1024 * 1024

	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{
			name:    "body smaller than limit",
			size:    1024,
			wantErr: nil,
		},
		{
			name:    "body exactly at limit",
			size:    maxSize,
			wantErr: nil,
		},
		{
			name:    "body over limit",
			size:    maxSize + 1,
			wantErr: ErrTooLargeResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &RSSParser{}
			body := io.NopCloser(strings.NewReader(strings.Repeat("x", tt.size)))

			data, err := p.readBody(body)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("readBody() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("readBody() unexpected error: %v", err)
			}

			if len(data) != tt.size {
				t.Errorf("readBody() len = %d, want %d", len(data), tt.size)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	const feedBody = `<rss><channel><title>News</title>` +
		`<item><title>First</title><link>https://example.com/1</link><pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate></item>` +
		`</channel></rss>`

	wantDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*60*60))

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantFeed   *RSSFeed
		wantErr    error
	}{
		{
			name:       "successful parse",
			statusCode: http.StatusOK,
			body:       feedBody,
			wantFeed: &RSSFeed{
				Channel: Channel{
					Title: "News",
					Items: []Item{
						{Title: "First", Link: "https://example.com/1", PubDate: "Mon, 02 Jan 2006 15:04:05 -0700", ParsedPubDate: &wantDate},
					},
				},
			},
		},
		{
			name:       "non 200 status",
			statusCode: http.StatusNotFound,
			body:       "not found",
			wantErr:    ErrHTTPStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			p := NewRSSParser(5 * time.Second)

			got, err := p.Parse(context.Background(), srv.URL)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Parse() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			if got == nil || tt.wantFeed == nil {
				t.Fatalf("Parse() got = %v, want %v", got, tt.wantFeed)
			}

			if got.Channel.Title != tt.wantFeed.Channel.Title {
				t.Errorf("Parse() title = %q, want %q", got.Channel.Title, tt.wantFeed.Channel.Title)
			}

			if len(got.Channel.Items) != len(tt.wantFeed.Channel.Items) {
				t.Fatalf("Parse() items count = %d, want %d", len(got.Channel.Items), len(tt.wantFeed.Channel.Items))
			}

			for i := range tt.wantFeed.Channel.Items {
				gotItem := &got.Channel.Items[i]
				wantItem := &tt.wantFeed.Channel.Items[i]

				if gotItem.Title != wantItem.Title {
					t.Errorf("Parse() items[%d].Title = %q, want %q", i, gotItem.Title, wantItem.Title)
				}

				if gotItem.ParsedPubDate == nil || !gotItem.ParsedPubDate.Equal(*wantItem.ParsedPubDate) {
					t.Errorf("Parse() items[%d].ParsedPubDate = %v, want %v", i, gotItem.ParsedPubDate, *wantItem.ParsedPubDate)
				}
			}
		})
	}

	t.Run("malformed xml", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "<rss><channel>")
		}))
		defer srv.Close()

		p := NewRSSParser(5 * time.Second)

		got, err := p.Parse(context.Background(), srv.URL)

		if err == nil {
			t.Fatalf("Parse() expected error for malformed XML, got nil (feed: %+v)", got)
		}
	})
}
