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
	AIConfig           OpenAIConfig
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

type OpenAIConfig struct {
	APIKey      string
	Model       string
	MaxTokens   int64
	Temperature float64
	Timeout     time.Duration
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

func loadAIConfig() (*OpenAIConfig, error) {
	apiKey := getEnv("OPENAI_API_KEY", "")
	model := getEnv("OPENAI_MODEL", "")
	maxTokensStr := getEnv("OPENAI_MAX_TOKENS", "0")
	temperatureStr := getEnv("OPENAI_TEMPERATURE", "0")
	timeoutStr := getEnv("OPENAI_TIMEOUT", "0")

	maxTokens, err := strconv.ParseInt(maxTokensStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid OPENAI_MAX_TOKENS: %w", err)
	}

	temperature, err := strconv.ParseFloat(temperatureStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid OPENAI_TEMPERATURE: %w", err)
	}

	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RABBIT_RETRY_DELAY: %w", err)
	}

	cfg := &OpenAIConfig{
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Timeout:     time.Duration(timeout) * time.Second,
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *OpenAIConfig) validate() error {
	var errs []string

	if c.APIKey == "" {
		errs = append(errs, "OPEN_API_KEY cannot be empty")
	}

	if c.Temperature < 0 || c.Temperature > 2 {
		errs = append(errs, "OPENAI_TEMPERATURE must be in 0 - 2.0 range")
	}

	if c.Timeout < 0 {
		errs = append(errs, "OPENAI_TIMEOUT cannot be negative")
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
