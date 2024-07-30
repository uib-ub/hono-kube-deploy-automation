package config

import (
	"fmt"
	"os"
)

type Config struct {
	GitHubToken   string
	WebhookSecret string
}

func LoadConfig() (*Config, error) {
	config := &Config{
		GitHubToken:   getEnv("GITHUB_TOKEN", ""),
		WebhookSecret: getEnv("WEBHOOK_SECRET", ""),
	}

	if config.WebhookSecret == "" {
		return nil, fmt.Errorf("missing required configuration: WEBHOOK_SECRET")
	}

	return config, nil
}

// getEnv fetches an environment variable or returns a default value if not set.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
