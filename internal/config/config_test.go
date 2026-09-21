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

func TestLoadConfig_EmptyScriptURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  allowed_user_ids:
    - 111111
installer:
  script_url: "   "
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error when script_url is empty, got nil")
	}
}

func TestLoadConfig_NegativeCooldown(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  allowed_user_ids:
    - 111111
limits:
  cooldown_per_user: -5m
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error when cooldown is negative, got nil")
	}
}

func TestLoadConfig_CommentPoolEnv(t *testing.T) {
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

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")
	t.Setenv("INSTALLER_COMMENT_POOL", "3,8,12")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Installer.CommentPool) != 3 || cfg.Installer.CommentPool[0] != 3 || cfg.Installer.CommentPool[1] != 8 || cfg.Installer.CommentPool[2] != 12 {
		t.Errorf("unexpected comment pool from env: %v", cfg.Installer.CommentPool)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
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

	t.Setenv("TG_BOT_TOKEN", "mock_token_123")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Installer.Timeout != 15*time.Minute {
		t.Errorf("expected default timeout 15m, got %v", cfg.Installer.Timeout)
	}

	expectedPool := []int{1, 2, 3, 4, 5, 6}
	if len(cfg.Installer.CommentPool) != len(expectedPool) {
		t.Fatalf("expected comment pool length %d, got %d", len(expectedPool), len(cfg.Installer.CommentPool))
	}
	for i, v := range expectedPool {
		if cfg.Installer.CommentPool[i] != v {
			t.Errorf("expected pool[%d]=%d, got %d", i, v, cfg.Installer.CommentPool[i])
		}
	}
}

