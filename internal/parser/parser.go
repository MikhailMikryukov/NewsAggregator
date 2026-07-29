package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RSSFeed struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title         string     `xml:"title"`
	Link          string     `xml:"link"`
	Description   string     `xml:"description"`
	PubDate       string     `xml:"pubDate"`
	ParsedPubDate *time.Time `xml:"-"` // Не парсится из XML
}

type RSSParser struct {
	client *http.Client
}

func NewRSSParser(timeout time.Duration) *RSSParser {
	return &RSSParser{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *RSSParser) Parse(ctx context.Context, url string) (*RSSFeed, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	body, err := p.fetchFeed(ctxWithTimeout, url)
	if err != nil {
		return nil, err
	}

	feed, err := p.parseFeed(body)
	if err != nil {
		return nil, err
	}

	if err := p.parseDates(feed); err != nil {
		return nil, err
	}

	return feed, nil
}

func (p *RSSParser) fetchFeed(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("User-Agent", "MyRSSReader/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status error: %d", resp.StatusCode)
	}

	return p.readBody(resp.Body)
}

func (p *RSSParser) readBody(body io.ReadCloser) ([]byte, error) {
	const maxSize = 10 * 1024 * 1024
	limited := io.LimitReader(body, maxSize)

	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("body reading error: %w", err)
	}

	if len(data) == maxSize {
		return nil, fmt.Errorf("response too large")
	}

	return data, nil
}

func (p *RSSParser) parseFeed(body []byte) (*RSSFeed, error) {
	var feed RSSFeed
	err := xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, fmt.Errorf("XML parsing error: %w", err)
	}
	return &feed, nil
}

func (p *RSSParser) parseDates(feed *RSSFeed) error {
	for i := range feed.Channel.Items {
		item := &feed.Channel.Items[i]
		if item.PubDate == "" {
			continue
		}

		parsedDate, err := parseRSSDate(item.PubDate)
		if err != nil {
			return fmt.Errorf("date parsing error %s: %w", item.Title, err)
		}
		item.ParsedPubDate = parsedDate
	}
	return nil
}

func parseRSSDate(dateStr string) (*time.Time, error) {
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dateStr)
		if err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("parsing data error '%s'", dateStr)
}
