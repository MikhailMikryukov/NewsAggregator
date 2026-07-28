package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"html/template"
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
	Title            string        `xml:"title"`
	Link             string        `xml:"link"`
	Description      string        `xml:"description"`
	PubDate          string        `xml:"pubDate"`
	ParsedPubDate    time.Time     `xml:"-"` // Не парсится из XML
	CleanDescription template.HTML `xml:"-"`
}

// RSSParser — структура, которая умеет парсить RSS
type RSSParser struct {
	client *http.Client
}

// NewRSSParser создает новый парсер
func NewRSSParser(timeout time.Duration) *RSSParser {
	return &RSSParser{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Parse - главный метод, который делает всю работу
func (p *RSSParser) Parse(ctx context.Context, url string) (*RSSFeed, error) {
	// Создаем запрос с контекстом (чтобы можно было отменить)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// Добавляем User-Agent (чтобы нас не банили)
	req.Header.Set("User-Agent", "MyRSSReader/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка HTTP статус: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения тела: %w", err)
	}

	var feed RSSFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга XML: %w", err)
	}

	// Обрабатываем каждый элемент
	for i := range feed.Channel.Items {
		item := &feed.Channel.Items[i]

		// Парсим дату
		if item.PubDate != "" {
			parsedDate, err := parseRSSDate(item.PubDate)
			if err != nil {
				return nil, fmt.Errorf("ошибка парсинга даты для %s: %w", item.Title, err)
			}
			item.ParsedPubDate = parsedDate
		}

		// Очищаем описание
		item.CleanDescription = cleanDescription(item.Description)
	}

	return &feed, nil
}

// parseRSSDate - теперь приватная функция внутри пакета parser
func parseRSSDate(dateStr string) (time.Time, error) {
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
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("не удалось распарсить дату '%s'", dateStr)
}

// cleanDescription - тоже приватная
func cleanDescription(desc string) template.HTML {
	return template.HTML(desc)
}
