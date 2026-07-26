package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("API_ROOT", "http://127.0.0.1:8212/")
	t.Setenv("API_PASSWORD", "api-secret")
	t.Setenv("ADMIN_PASSWORD", "admin-secret")
	t.Setenv("PUBLIC_READ", "true")
	t.Setenv("DATA_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("GAME_DATA_ENABLED", "true")
	t.Setenv("GAME_DATA_POLL_INTERVAL", "2m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIRoot != "http://127.0.0.1:8212" {
		t.Fatalf("APIRoot = %q", cfg.APIRoot)
	}
	if !cfg.PublicRead {
		t.Fatal("PublicRead = false")
	}
	if cfg.PollInterval != 30*time.Second {
		t.Fatalf("PollInterval = %v", cfg.PollInterval)
	}
	if !cfg.GameDataEnabled || cfg.GameDataInterval != 2*time.Minute {
		t.Fatalf("game data config = %v, %v", cfg.GameDataEnabled, cfg.GameDataInterval)
	}
}

func TestLoadRejectsFastGameDataPolling(t *testing.T) {
	t.Setenv("API_ROOT", "http://127.0.0.1:8212")
	t.Setenv("API_PASSWORD", "api-secret")
	t.Setenv("ADMIN_PASSWORD", "admin-secret")
	t.Setenv("DATA_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("GAME_DATA_ENABLED", "true")
	t.Setenv("GAME_DATA_POLL_INTERVAL", "5s")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("API_ROOT", "")
	t.Setenv("API_PASSWORD", "")
	t.Setenv("ADMIN_PASSWORD", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadAcceptsDataDirectory(t *testing.T) {
	t.Setenv("API_ROOT", "http://127.0.0.1:8212")
	t.Setenv("API_PASSWORD", "api-secret")
	t.Setenv("ADMIN_PASSWORD", "admin-secret")
	t.Setenv("DATA_PATH", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if filepath.Base(cfg.DataPath) != "pal-pal.db" {
		t.Fatalf("DataPath = %q", cfg.DataPath)
	}
}
