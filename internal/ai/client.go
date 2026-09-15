package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yandex-cloud/go-sdk/v2/pkg/endpoints"
	"github.com/yandex-cloud/go-sdk/v2/pkg/iamkey"
	"io"
	"strings"
	"time"

	v1 "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/foundation_models/v1"
	foundation_models "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/foundation_models/v1/text_generation"
	"github.com/yandex-cloud/go-sdk/v2"
	"github.com/yandex-cloud/go-sdk/v2/credentials"
	"github.com/yandex-cloud/go-sdk/v2/pkg/options"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/MikhailMikryukov/NewsAggregator/internal/config"
)

var (
	ErrEmptyResponseAI = errors.New("empty response from AI")
	ErrJSONNotFound    = errors.New("JSON response not found")
)

type Tagger interface {
	GenerateTags(ctx context.Context, req TagRequest) (*TagResponse, error)
	HealthCheck(ctx context.Context) error
}

type TagRequest struct {
	Description string `json:"description"`
	Title       string `json:"title,omitempty"`
	MaxTags     int    `json:"max_tags,omitempty"`
}

type TagResponse struct {
	Tags []string `json:"tags"`
}

type YandexAIClient struct {
	sdk *ycsdk.SDK
	cfg config.AIConfig
}

func NewYandexAIClient(ctx context.Context, cfg config.AIConfig) (*YandexAIClient, error) {

	if cfg.Temperature == 0 {
		cfg.Temperature = 0.1
	}

	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 200
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	key, err := iamkey.ReadFromJSONFile(cfg.YandexKeyFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading key file: %w", err)
	}

	creds, err := credentials.ServiceAccountKey(key)
	if err != nil {
		return nil, fmt.Errorf("error creating credentials: %w", err)
	}

	resolver := endpoints.NewPrefixEndpointsResolver(endpoints.PrefixToEndpoint{
		"/yandex.cloud.ai": endpoints.NewEndpointParams("ai.api.cloud.yandex.net"),
		"yandex.cloud.iam": endpoints.NewEndpointParams("iam.api.cloud.yandex.net"),
	})

	sdk, err := ycsdk.Build(ctx,
		options.WithCredentials(creds),
		options.WithEndpointsResolver(resolver),
	)

	if err != nil {
		return nil, fmt.Errorf("error building sdk: %w", err)
	}

	return &YandexAIClient{
		sdk: sdk,
		cfg: cfg,
	}, nil
}

func (c *YandexAIClient) GenerateTags(ctx context.Context, req TagRequest) (*TagResponse, error) {
	delay := c.cfg.Retry.Delay
	backoff := c.cfg.Retry.Backoff

	var resp *TagResponse
	var err error

	for i := 0; i < c.cfg.Retry.Attempts; i++ {

		resp, err = c.doRequest(ctx, req)
		if err != nil && !errors.Is(err, context.Canceled) {
			time.Sleep(delay)
			delay = delay * time.Duration(backoff)
			continue
		} else {
			break
		}
	}

	return resp, err
}

func (c *YandexAIClient) doRequest(ctx context.Context, req TagRequest) (*TagResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	conn, err := c.sdk.GetConnection(ctxTimeout, foundation_models.TextGenerationService_Completion_FullMethodName)
	if err != nil {
		return nil, fmt.Errorf("getting connection error: %w", err)
	}

	client := foundation_models.NewTextGenerationServiceClient(conn)

	modelURI := fmt.Sprintf("gpt://%s/%s", c.cfg.YandexFolderID, c.cfg.YandexModel)

	var systemMsg v1.Message
	systemMsg.SetRole("system")
	systemMsg.SetText(c.getSystemPrompt())

	var userMsg v1.Message
	userMsg.SetRole("user")
	userMsg.SetText(c.buildPrompt(req, c.cfg.MaxTags))

	cr := &foundation_models.CompletionRequest{
		ModelUri: modelURI,
		CompletionOptions: &v1.CompletionOptions{
			Stream:      false,
			Temperature: wrapperspb.Double(c.cfg.Temperature),
			MaxTokens:   wrapperspb.Int64(c.cfg.MaxTokens),
		},
		Messages: []*v1.Message{
			&systemMsg,
			&userMsg,
		},
	}

	stream, err := client.Completion(ctxTimeout, cr)
	if err != nil {
		return nil, fmt.Errorf("error stream creating: %w", err)
	}

	rec, err := stream.Recv()
	if err != nil || len(rec.GetAlternatives()) == 0 {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyResponseAI
		}
		return nil, fmt.Errorf("error getting response from AI: %w", err)
	}

	if len(rec.GetAlternatives()) == 0 {
		return nil, ErrEmptyResponseAI
	}

	content := rec.GetAlternatives()[0].GetMessage().GetText()

	return c.parseResponse(content)
}

func (c *YandexAIClient) HealthCheck(ctx context.Context) error {
	testReq := TagRequest{
		Description: "Тестовое описание для проверки работоспособности",
		MaxTags:     1,
	}

	_, err := c.GenerateTags(ctx, testReq)
	if err != nil {
		return fmt.Errorf("health check failed")
	}

	return nil
}

func (c *YandexAIClient) buildPrompt(req TagRequest, maxTags int) string {
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

func (c *YandexAIClient) getSystemPrompt() string {
	return "Ты - профессиональный контент-менеджер с 10-летним опытом. " +
		"Ты умеешь точно выделять ключевые темы из текста. " +
		"Твои ответы всегда структурированы и точны."
}

func (c *YandexAIClient) parseResponse(content string) (*TagResponse, error) {
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
