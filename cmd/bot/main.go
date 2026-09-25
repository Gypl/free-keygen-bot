package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"telegram-deploy-bot/internal/bot"
	"telegram-deploy-bot/internal/config"
	"telegram-deploy-bot/internal/installer"
	"telegram-deploy-bot/internal/logging"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "Path to YAML configuration file")
	flag.Parse()

	// 1. Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize structured logging
	logger := logging.NewLogger(cfg.Logging.Level, cfg.Logging.JSON)
	slog.SetDefault(logger)
	auditLogger := logging.NewAuditLogger(logger)

	logger.Info("starting telegram vpn deploy bot",
		"execution_mode", cfg.App.ExecutionMode,
		"allowed_users_count", len(cfg.Telegram.AllowedUserIDs),
		"installer_timeout", cfg.Installer.Timeout,
		"cooldown", cfg.Limits.CooldownPerUser,
	)

	// 3. Initialize domain services
	inst := installer.New(
		cfg.Installer.ScriptURL,
		cfg.Installer.Timeout,
		cfg.Installer.CommentPool,
		cfg.Installer.CommentSuffix,
		logger,
	)

	// 4. Initialize bot handler
	handler := bot.NewHandler(
		nil, // Replaced inside bot.New with real tgbotapi.BotAPI
		cfg.Telegram.AllowedUserIDs,
		cfg.Limits.CooldownPerUser,
		inst,
		logger,
		auditLogger,
	)

	// 5. Initialize Telegram bot client
	tgBot, err := bot.New(cfg.Telegram.BotToken, handler, logger)
	if err != nil {
		logger.Error("failed to initialize telegram bot", "error", err)
		os.Exit(1)
	}

	// 6. Run with graceful shutdown on interrupt
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := tgBot.Start(ctx); err != nil {
		logger.Error("bot stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("telegram bot stopped cleanly")
}
