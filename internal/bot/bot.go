package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot manages the Telegram bot lifecycle, polling, and dispatching.
type Bot struct {
	api     *tgbotapi.BotAPI
	handler *Handler
	logger  *slog.Logger
}

// New creates a new Bot instance.
func New(token string, handler *Handler, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot api: %w", err)
	}

	// Update handler's sender with the real BotAPI
	handler.sender = api

	return &Bot{
		api:     api,
		handler: handler,
		logger:  logger,
	}, nil
}

// Start begins receiving updates from Telegram using long polling until the context is canceled.
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("authorized on account", "username", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)

	b.logger.Info("bot polling started")

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("shutting down bot polling...")
			b.api.StopReceivingUpdates()
			return nil

		case update, ok := <-updates:
			if !ok {
				b.logger.Info("updates channel closed")
				return nil
			}

			// Process each update in a separate goroutine so slow installs don't block other commands
			go func(up tgbotapi.Update) {
				defer func() {
					if r := recover(); r != nil {
						b.logger.Error("panic in update handler", "panic", r, "update_id", up.UpdateID)
					}
				}()
				b.handler.HandleUpdate(ctx, up)
			}(update)
		}
	}
}
