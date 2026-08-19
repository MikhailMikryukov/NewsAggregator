package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/MikhailMikryukov/NewsAggregator/internal/config"
)

var (
	ErrDecryptionEmpty = errors.New("description cannot be empty")
	ErrEmptyResponseAI = errors.New("empty response from OpenAI")
	ErrJSONNotFound    = errors.New("JSON response not found")
)

type TagRequest struct {
	Description string `json:"description"`
	Title       string `json:"title,omitempty"`
	MaxTags     int    `json:"max_tags,omitempty"`
}

type TagResponse struct {
	Tags []string `json:"tags"`
}

type OpenAIClient struct {
	client openai.Client
	config config.OpenAIConfig
}

func NewOpenAIClient(cfg config.OpenAIConfig) *OpenAIClient {
	if cfg.Model == "" {
		cfg.Model = openai.ChatModelGPT4oMini
	}

	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}

	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 200
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OpenAIClient{
		client: openai.NewClient(
			option.WithAPIKey(cfg.APIKey),
		),
		config: cfg,
	}
}

func (c *OpenAIClient) GenerateTags(ctx context.Context, req TagRequest) (*TagResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	if req.Description == "" {
		return nil, ErrDecryptionEmpty
	}

	maxTags := req.MaxTags
	if maxTags == 0 {
		maxTags = 5
	}

	prompt := c.buildPrompt(req, maxTags)

	chatCompletion, err := c.client.Chat.Completions.New(ctxTimeout, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(c.getSystemPrompt()),
			openai.UserMessage(prompt),
		},
		Model:       c.config.Model,
		Temperature: openai.Float(c.config.Temperature),
		MaxTokens:   openai.Int(c.config.MaxTokens),
	})

	if err != nil {
		return nil, fmt.Errorf("OpenAI request error: %w", err)
	}

	if len(chatCompletion.Choices) == 0 {
		return nil, ErrEmptyResponseAI
	}

	return c.parseResponse(chatCompletion.Choices[0].Message.Content)
}

func (c *OpenAIClient) HealthCheck(ctx context.Context) error {
	_, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("ping"),
		},
		Model:     c.config.Model,
		MaxTokens: openai.Int(5),
	})

	return err
}

func (c *OpenAIClient) buildPrompt(req TagRequest, maxTags int) string {
	var prompt strings.Builder

	prompt.WriteString("Проанализируй описание статьи и выдели ключевые теги.\n")
	fmt.Fprintf(&prompt, "Максимальное количество тегов: %d\n", maxTags)
	prompt.WriteString("Теги должны быть на русском языке, отражать суть статьи.\n")
	prompt.WriteString("Теги должны быть разделены запятыми.\n\n")

	if req.Title != "" {
		fmt.Fprintf(&prompt, "Заголовок статьи: %s\n", req.Title)
	}

	fmt.Fprintf(&prompt, "Описание: %s\n\n", req.Description)
	prompt.WriteString(`Ответ дай в формате JSON:
{"tags": ["тег1", "тег2", "тег3"]}`)

	return prompt.String()
}

func (c *OpenAIClient) getSystemPrompt() string {
	return "Ты - профессиональный контент-менеджер с 10-летним опытом. " +
		"Ты умеешь точно выделять ключевые темы из текста. " +
		"Твои ответы всегда структурированы и точны."
}

func (c *OpenAIClient) parseResponse(content string) (*TagResponse, error) {
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("%w: %s", ErrJSONNotFound, content)
	}

	jsonStr := content[start : end+1]

	var result TagResponse

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %w", err)
	}

	return &result, nil
}
