package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIRoot        string
	APIPassword    string
	PublicRead     bool
	PublicPassword string
	AdminPassword  string
	DataPath       string
	ListenAddress  string
	SecureCookies  bool
	PollInterval   time.Duration
	HistoryLimit   time.Duration
}

func Load() (Config, error) {
	publicRead, err := envBool("PUBLIC_READ", false)
	if err != nil {
		return Config{}, err
	}

	secureCookies, err := envBool("SECURE_COOKIES", false)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := envDuration("POLL_INTERVAL", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	historyLimit, err := envDuration("HISTORY_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		APIRoot:        strings.TrimRight(strings.TrimSpace(os.Getenv("API_ROOT")), "/"),
		APIPassword:    os.Getenv("API_PASSWORD"),
		PublicRead:     publicRead,
		PublicPassword: os.Getenv("PUBLIC_PASSWORD"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		DataPath:       envOr("DATA_PATH", "data"),
		ListenAddress:  envOr("LISTEN_ADDRESS", ":8080"),
		SecureCookies:  secureCookies,
		PollInterval:   pollInterval,
		HistoryLimit:   historyLimit,
	}

	var missing []string
	if cfg.APIRoot == "" {
		missing = append(missing, "API_ROOT")
	}
	if cfg.APIPassword == "" {
		missing = append(missing, "API_PASSWORD")
	}
	if cfg.AdminPassword == "" {
		missing = append(missing, "ADMIN_PASSWORD")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if cfg.PollInterval < time.Second {
		return Config{}, errors.New("POLL_INTERVAL must be at least 1s")
	}
	if cfg.HistoryLimit < time.Hour {
		return Config{}, errors.New("HISTORY_RETENTION must be at least 1h")
	}

	cfg.DataPath, err = filepath.Abs(cfg.DataPath)
	if err != nil {
		return Config{}, fmt.Errorf("resolve DATA_PATH: %w", err)
	}
	if filepath.Ext(cfg.DataPath) == "" {
		cfg.DataPath = filepath.Join(cfg.DataPath, "pal-pal.db")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 15s or 720h: %w", key, err)
	}
	return parsed, nil
}
