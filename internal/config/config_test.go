package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  allowed_user_ids:
    - 111111
    - 222222

installer:
  script_url: "https://example.com/install.sh"
  timeout: 3m
  comment_pool: [2, 3, 5]
  comment_suffix: "test"

limits:
  cooldown_per_user: 5m

logging:
  level: "debug"
  json: false
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Telegram.BotToken != "mock_token_123" {
		t.Errorf("expected token mock_token_123, got %s", cfg.Telegram.BotToken)
	}

	if len(cfg.Telegram.AllowedUserIDs) != 2 || cfg.Telegram.AllowedUserIDs[0] != 111111 {
		t.Errorf("unexpected allowed_user_ids: %v", cfg.Telegram.AllowedUserIDs)
	}

	if cfg.Installer.Timeout != 3*time.Minute {
		t.Errorf("expected timeout 3m, got %v", cfg.Installer.Timeout)
	}

	if cfg.Installer.CommentSuffix != "test" {
		t.Errorf("expected suffix test, got %s", cfg.Installer.CommentSuffix)
	}
}

func TestLoadConfig_MissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  allowed_user_ids:
    - 111111
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_ = os.Unsetenv("TG_BOT_TOKEN")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error when TG_BOT_TOKEN is missing, got nil")
	}
}

func TestLoadConfig_EmptyAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  allowed_user_ids: []
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error when allowlist is empty, got nil")
	}
}
