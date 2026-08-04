package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DBConnectionString string
	RssWorkersNum      int
	RabbitCfg          RabbitConfig
}

type RabbitConfig struct {
	URL                string
	ConnectionTimeout  time.Duration
	Heartbeat          time.Duration
	ReconnectStrategy  RetryStrategy
	PublishingStrategy RetryStrategy
	ConsumingStrategy  RetryStrategy
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
	rabbitAddress := getEnv("RABBIT_ADDRESS", "")
	rabbitTimeoutStr := getEnv("RABBIT_TIMEOUT", "0")
	rabbitHeartbeatStr := getEnv("RABBIT_HEARTBEAT", "0")
	retryAttemptsStr := getEnv("RABBIT_RETRY_ATTEMPTS", "0")
	retryDelayStr := getEnv("RABBIT_RETRY_DELAY", "0")
	retryBackoffStr := getEnv("RABBIT_RETRY_BACKOFF", "0")

	workersNum, err := strconv.Atoi(workersNumStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	retryAttempts, err := strconv.Atoi(retryAttemptsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	rabbitTimeout, err := strconv.Atoi(rabbitTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	rabbitHeartbeat, err := strconv.Atoi(rabbitHeartbeatStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	retryDelay, err := strconv.Atoi(retryDelayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
	}

	retryBackoff, err := strconv.ParseFloat(retryBackoffStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RSS_WORKERS_NUM: %w", err)
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
		ReconnectStrategy:  strategy,
		PublishingStrategy: strategy,
		ConsumingStrategy:  strategy,
	}

	cfg := &Config{
		Port:               port,
		DBConnectionString: connStr,
		RssWorkersNum:      workersNum,
		RabbitCfg:          rabbitCfg,
	}

	if err = validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	var errs []string

	if cfg.Port == "" {
		errs = append(errs, "PORT cannot be empty")
	}
	if portNum, err := strconv.Atoi(cfg.Port); err == nil {
		if portNum < 1 || portNum > 65535 {
			errs = append(errs, fmt.Sprintf("PORT must be between 1 and 65535, got %s", cfg.Port))
		}
	}

	if cfg.DBConnectionString == "" {
		errs = append(errs, "DATABASE_URL cannot be empty")
	}
	if !strings.HasPrefix(cfg.DBConnectionString, "postgresql://") {
		errs = append(errs, "DATABASE_URL must be a valid postgresql connection string")
	}

	if cfg.RabbitCfg.URL == "" {
		errs = append(errs, "RABBIT_ADDRESS cannot be empty")
	}

	if cfg.RssWorkersNum <= 0 {
		errs = append(errs, fmt.Sprintf("RSS_WORKERS_NUM must be positive, got %d", cfg.RssWorkersNum))
	}
	if cfg.RssWorkersNum > 100 {
		errs = append(errs, fmt.Sprintf("RSS_WORKERS_NUM is too high (%d), maximum is 100", cfg.RssWorkersNum))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
