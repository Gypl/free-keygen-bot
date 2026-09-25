package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App struct {
		ExecutionMode string `yaml:"execution_mode" env:"EXECUTION_MODE" env-default:"docker"`
	} `yaml:"app"`

	Telegram struct {
		BotToken       string  `yaml:"-" env:"TG_BOT_TOKEN" env-required:"true"`
		AllowedUserIDs []int64 `yaml:"allowed_user_ids" env:"TG_ALLOWED_USER_IDS" env-separator:","`
	} `yaml:"telegram"`

	Installer struct {
		ScriptURL     string        `yaml:"script_url" env:"INSTALLER_SCRIPT_URL" env-default:"https://raw.githubusercontent.com/openlibrecommunity/olcrtc/master/install.sh"`
		Timeout       time.Duration `yaml:"timeout" env:"INSTALLER_TIMEOUT" env-default:"15m"`
		CommentPool   []int         `yaml:"comment_pool" env:"INSTALLER_COMMENT_POOL" env-separator:","`
		CommentSuffix string        `yaml:"comment_suffix" env:"INSTALLER_COMMENT_SUFFIX" env-default:"welcome"`
	} `yaml:"installer"`

	Limits struct {
		CooldownPerUser time.Duration `yaml:"cooldown_per_user" env:"LIMITS_COOLDOWN_PER_USER" env-default:"10m"`
	} `yaml:"limits"`

	Logging struct {
		Level string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
		JSON  bool   `yaml:"json" env:"LOG_JSON" env-default:"true"`
	} `yaml:"logging"`
}

// Load loads the configuration from the YAML file at configPath,
// overriding with environment variables. A .env file is optionally loaded if present.
func Load(configPath string) (*Config, error) {
	// Optionally load .env file; ignore if missing
	_ = godotenv.Load()

	var cfg Config

	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
				return nil, fmt.Errorf("read config file: %w", err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat config file: %w", err)
		} else {
			// File does not exist, read purely from env
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				return nil, fmt.Errorf("read env: %w", err)
			}
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("read env: %w", err)
		}
	}

	// Invariant validations
	if cfg.App.ExecutionMode != "docker" && cfg.App.ExecutionMode != "systemd" {
		return nil, fmt.Errorf("config: invalid execution_mode %q, expected 'docker' or 'systemd'", cfg.App.ExecutionMode)
	}

	if cfg.Telegram.BotToken == "" {
		return nil, errors.New("config: TG_BOT_TOKEN is required and cannot be empty")
	}

	if len(cfg.Telegram.AllowedUserIDs) == 0 {
		return nil, errors.New("config: allowed_user_ids cannot be empty")
	}

	if strings.TrimSpace(cfg.Installer.ScriptURL) == "" {
		return nil, errors.New("config: installer script_url cannot be empty")
	}

	if cfg.Installer.Timeout <= 0 {
		return nil, errors.New("config: installer timeout must be positive")
	}

	if cfg.Limits.CooldownPerUser < 0 {
		return nil, errors.New("config: cooldown must be non-negative")
	}

	if len(cfg.Installer.CommentPool) == 0 {
		cfg.Installer.CommentPool = []int{1, 2, 3, 4, 5, 6}
	}

	if cfg.Installer.CommentSuffix == "" {
		cfg.Installer.CommentSuffix = "welcome"
	}

	return &cfg, nil
}
