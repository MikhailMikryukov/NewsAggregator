package workers

import (
	"NewsAggregator/internal/repository"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"
)

// RSSFeed представляет корневой элемент <rss>
type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel представляет элемент <channel>
type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

// Item представляет элемент <item>
type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`

	// Парсенные поля (не из XML)
	ParsedPubDate    time.Time
	CleanDescription template.HTML // Для безопасного вывода в HTML
}

type ParseWorker struct {
	repo repository.ISourceArticleRepository
}

// Получить и распарсить RSS ленту
func (pw *ParseWorker) fetchRSSFeed(url string) (*RSSFeed, error) {
	// Делаем HTTP-запрос
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка HTTP статус: %d", resp.StatusCode)
	}

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения тела: %w", err)
	}

	// Парсим XML
	var feed RSSFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга XML: %w", err)
	}

	// Парсим даты и очищаем description для каждого элемента
	for i := range feed.Channel.Items {
		item := &feed.Channel.Items[i]

		// Парсим дату
		if item.PubDate != "" {
			parsedDate, err := parseRSSDate(item.PubDate)
			if err != nil {
				// Логируем ошибку, но продолжаем работу
				fmt.Printf("Предупреждение: %v\n", err)
				item.ParsedPubDate = time.Time{} // Zero time
			} else {
				item.ParsedPubDate = parsedDate
			}
		}

		// Очищаем description для безопасного вывода
		item.CleanDescription = cleanDescription(item.Description)

	}

	return &feed, nil
}

// Парсит дату в формате RSS (RFC 822)
func parseRSSDate(dateStr string) (time.Time, error) {
	// Стандартный формат RSS: "Mon, 02 Jan 2006 15:04:05 MST"
	layouts := []string{
		time.RFC1123Z,               // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,                // "Mon, 02 Jan 2006 15:04:05 MST"
		time.RFC822Z,                // "02 Jan 06 15:04 -0700"
		time.RFC822,                 // "02 Jan 06 15:04 MST"
		"2006-01-02T15:04:05Z",      // ISO 8601
		"2006-01-02T15:04:05-07:00", // ISO 8601 с таймзоной
	}

	var err error
	for _, layout := range layouts {
		var t time.Time
		t, err = time.Parse(layout, dateStr)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("не удалось распарсить дату '%s': %w", dateStr, err)
}

// Возвращает безопасный HTML
func cleanDescription(desc string) template.HTML {
	return template.HTML(desc)
}
