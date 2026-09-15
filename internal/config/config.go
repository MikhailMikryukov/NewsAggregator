package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	ErrConfigValidation       = errors.New("config validation failed")
	ErrRabbitConfigValidation = errors.New("rabbit config validation failed")
	ErrAIConfigValidation     = errors.New("open ai config validation failed")
)

type Config struct {
	Port               string
	DBConnectionString string
	AIConfig           AIConfig
	RabbitCfg          RabbitConfig
	RssWorkersNum      int
}

type RabbitConfig struct {
	URL                string
	ConnectionTimeout  time.Duration
	Heartbeat          time.Duration
	ConsumerWorkersNum int
	ReconnectStrategy  RetryStrategy
	PublishingStrategy RetryStrategy
	ConsumingStrategy  RetryStrategy
}

type AIConfig struct {
	YandexKeyFilePath string
	YandexFolderID    string
	YandexModel       string
	MaxTokens         int64
	Temperature       float64
	MaxTags           int
	Timeout           time.Duration
	Retry             RetryStrategy
}

type RetryStrategy struct {
	Attempts int
	Delay    time.Duration
	Backoff  float64
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	port := getEnv("PORT", "8080")
	connStr := getEnv("DATABASE_URL", "postgresql://login:password@localhost:5432/")
	workersNumStr := getEnv("RSS_WORKERS_NUM", "0")

	workersNum, err := strconv.Atoi(workersNumStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	rabbitCfg, err := loadRabbitConfig()
	if err != nil {
		return nil, err
	}

	aiCfg, err := loadAIConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:               port,
		DBConnectionString: connStr,
		RssWorkersNum:      workersNum,
		RabbitCfg:          *rabbitCfg,
		AIConfig:           *aiCfg,
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadAIConfig() (*AIConfig, error) {
	YandexKeyFilePath := getEnv("YANDEX_GPT_KEY_FILE_PATH", "")
	model := getEnv("YANDEX_GPT_MODEL", "yandexgpt-lite")
	folderId := getEnv("YANDEX_GPT_FOLDER_ID", "")
	maxTokensStr := getEnv("YANDEX_GPT_MAX_TOKENS", "0")
	temperatureStr := getEnv("YANDEX_GPT_TEMPERATURE", "0")
	timeoutStr := getEnv("YANDEX_GPT_TIMEOUT", "0")
	maxTagsStr := getEnv("YANDEX_GPT_MAX_TAGS", "5")
	retryAttemptsStr := getEnv("YANDEX_GPT_RETRY_ATTEMPTS", "3")
	retryDelayStr := getEnv("YANDEX_GPT_RETRY_DELAY", "1")
	retryBackoffStr := getEnv("YANDEX_GPT_RETRY_BACKOFF", "2")

	maxTokens, err := strconv.ParseInt(maxTokensStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_MAX_TOKENS: %w", err)
	}

	temperature, err := strconv.ParseFloat(temperatureStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_TEMPERATURE: %w", err)
	}

	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_TIMEOUT: %w", err)
	}

	maxTags, err := strconv.Atoi(maxTagsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_MAX_TAGS: %w", err)
	}

	retryAttempts, err := strconv.Atoi(retryAttemptsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_RETRY_ATTEMPTS: %w", err)
	}

	retryDelay, err := strconv.Atoi(retryDelayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_RETRY_DELAY: %w", err)
	}

	retryBackoff, err := strconv.Atoi(retryBackoffStr)
	if err != nil {
		return nil, fmt.Errorf("invalid YANDEX_GPT_RETRY_BACKOFF: %w", err)
	}

	cfg := &AIConfig{
		YandexKeyFilePath: YandexKeyFilePath,
		YandexModel:       model,
		YandexFolderID:    folderId,
		MaxTokens:         maxTokens,
		Temperature:       temperature,
		MaxTags:           maxTags,
		Timeout:           time.Duration(timeout) * time.Second,
		Retry: RetryStrategy{
			Attempts: retryAttempts,
			Delay:    time.Duration(retryDelay),
			Backoff:  float64(retryBackoff),
		},
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *AIConfig) validate() error {
	var errs []string

	if c.YandexKeyFilePath == "" {
		errs = append(errs, "YANDEX_GPT_KEY_FILE_PATH cannot be empty")
	}

	if c.YandexModel == "" {
		errs = append(errs, "YANDEX_GPT_MODEL cannot be empty")
	}

	if c.YandexFolderID == "" {
		errs = append(errs, "YANDEX_GPT_FOLDER_ID cannot be empty")
	}

	if c.Temperature < 0 || c.Temperature > 1 {
		errs = append(errs, "YANDEX_GPT_TEMPERATURE must be in 0 - 1.0 range")
	}

	if c.Timeout < 0 {
		errs = append(errs, "YANDEX_GPT_TIMEOUT cannot be negative")
	}

	if c.Retry.Attempts < 0 {
		errs = append(errs, "YANDEX_GPT_RETRY_ATTEMPTS cannot be negative")
	}

	if c.Retry.Delay < 0 {
		errs = append(errs, "YANDEX_GPT_RETRY_DELAY cannot be negative")
	}

	if c.Retry.Backoff < 0 {
		errs = append(errs, "YANDEX_GPT_RETRY_BACKOFF cannot be negative")
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w :\n%s", ErrAIConfigValidation, strings.Join(errs, "\n"))
	}

	return nil
}

func loadRabbitConfig() (*RabbitConfig, error) {
	rabbitAddress := getEnv("RABBIT_ADDRESS", "")
	rabbitTimeoutStr := getEnv("RABBIT_TIMEOUT", "0")
	rabbitHeartbeatStr := getEnv("RABBIT_HEARTBEAT", "0")
	retryAttemptsStr := getEnv("RABBIT_RETRY_ATTEMPTS", "0")
	retryDelayStr := getEnv("RABBIT_RETRY_DELAY", "0")
	retryBackoffStr := getEnv("RABBIT_RETRY_BACKOFF", "0")
	rabbitWorkersNumStr := getEnv("RABBIT_CONSUMER_WORKERS_NUM", "0")

	rabbitWorkersNum, err := strconv.Atoi(rabbitWorkersNumStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_CONSUMER_WORKERS_NUM: %w", err)
	}

	retryAttempts, err := strconv.Atoi(retryAttemptsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_RETRY_ATTEMPTS: %w", err)
	}

	rabbitTimeout, err := strconv.Atoi(rabbitTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_TIMEOUT: %w", err)
	}

	rabbitHeartbeat, err := strconv.Atoi(rabbitHeartbeatStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_HEARTBEAT: %w", err)
	}

	retryDelay, err := strconv.Atoi(retryDelayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_RETRY_DELAY: %w", err)
	}

	retryBackoff, err := strconv.ParseFloat(retryBackoffStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_RETRY_BACKOFF: %w", err)
	}

	strategy := RetryStrategy{
		Attempts: retryAttempts,
		Delay:    time.Duration(retryDelay) * time.Second,
		Backoff:  retryBackoff,
	}

	rabbitCfg := RabbitConfig{
		URL:                rabbitAddress,
		ConnectionTimeout:  time.Duration(rabbitTimeout) * time.Second,
		Heartbeat:          time.Duration(rabbitHeartbeat) * time.Second,
		ConsumerWorkersNum: rabbitWorkersNum,
		ReconnectStrategy:  strategy,
		PublishingStrategy: strategy,
		ConsumingStrategy:  strategy,
	}

	err = rabbitCfg.validate()
	if err != nil {
		return nil, err
	}

	return &rabbitCfg, nil
}

func (c *RabbitConfig) validate() error {
	var errs []string

	if c.URL == "" {
		errs = append(errs, "RABBIT_ADDRESS cannot be empty")
	}

	if c.ConnectionTimeout < 0 {
		errs = append(errs, "RABBIT_TIMEOUT cannot be negative")
	}

	if c.Heartbeat < 0 {
		errs = append(errs, "RABBIT_HEARTBEAT cannot be negative")
	}

	if c.ConsumerWorkersNum < 1 {
		errs = append(errs, "RABBIT_CONSUMER_WORKERS_NUM cannot be less than 1")
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w :\n%s", ErrRabbitConfigValidation, strings.Join(errs, "\n"))
	}

	return nil
}

func (c *Config) validate() error {
	var errs []string

	if c.Port == "" {
		errs = append(errs, "PORT cannot be empty")
	}
	if portNum, err := strconv.Atoi(c.Port); err == nil {
		if portNum < 1 || portNum > 65535 {
			errs = append(errs, fmt.Sprintf("PORT must be between 1 and 65535, got %s", c.Port))
		}
	}

	if c.DBConnectionString == "" {
		errs = append(errs, "DATABASE_URL cannot be empty")
	}
	if !strings.HasPrefix(c.DBConnectionString, "postgresql://") {
		errs = append(errs, "DATABASE_URL must be a valid postgresql connection string")
	}

	if c.RssWorkersNum <= 0 {
		errs = append(errs, fmt.Sprintf("RSS_WORKERS_NUM must be positive, got %d", c.RssWorkersNum))
	}
	if c.RssWorkersNum > 100 {
		errs = append(errs, fmt.Sprintf("RSS_WORKERS_NUM is too high (%d), maximum is 100", c.RssWorkersNum))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w :\n%s", ErrConfigValidation, strings.Join(errs, "\n"))
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
