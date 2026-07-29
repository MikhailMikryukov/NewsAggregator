package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	DBConnectionString string
	RssWorkersNum      int
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
	cfg := &Config{
		Port:               port,
		DBConnectionString: connStr,
		RssWorkersNum:      workersNum,
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
