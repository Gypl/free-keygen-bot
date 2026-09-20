package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot manages the Telegram bot lifecycle, polling, and dispatching.
type Bot struct {
	api     *tgbotapi.BotAPI
	handler *Handler
	logger  *slog.Logger
	wg      sync.WaitGroup
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

loop:
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("shutting down bot polling...")
			b.api.StopReceivingUpdates()
			break loop

		case update, ok := <-updates:
			if !ok {
				b.logger.Info("updates channel closed")
				break loop
			}

			// Process each update in a separate goroutine so slow installs don't block other commands
			b.wg.Add(1)
			go func(up tgbotapi.Update) {
				defer b.wg.Done()
				defer func() {
					if r := recover(); r != nil {
						b.logger.Error("panic in update handler", "panic", r, "update_id", up.UpdateID)
					}
				}()
				b.handler.HandleUpdate(ctx, up)
			}(update)
		}
	}

	// Wait for any in-flight update handlers to complete
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("all in-flight updates completed")
	case <-time.After(15 * time.Second):
		b.logger.Warn("timeout waiting for in-flight updates to complete")
	}

	return nil
}
