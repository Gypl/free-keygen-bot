package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-deploy-bot/internal/logging"
)

// Deployer abstracts the VPN installation operation.
type Deployer interface {
	Deploy(ctx context.Context) (string, error)
}

// MessageSender abstracts tgbotapi.BotAPI message sending for easy unit testing.
type MessageSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Handler handles incoming Telegram updates and commands.
type Handler struct {
	sender       MessageSender
	allowedUsers map[int64]struct{}
	cooldown     time.Duration
	cooldowns    sync.Map // map[int64]time.Time
	installLock  sync.Mutex
	deployer     Deployer
	logger       *slog.Logger
	auditLogger  *logging.AuditLogger
}

// NewHandler creates a new command Handler.
func NewHandler(
	sender MessageSender,
	allowedUserIDs []int64,
	cooldown time.Duration,
	deployer Deployer,
	logger *slog.Logger,
	auditLogger *logging.AuditLogger,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if auditLogger == nil {
		auditLogger = logging.NewAuditLogger(logger)
	}

	allowed := make(map[int64]struct{}, len(allowedUserIDs))
	for _, id := range allowedUserIDs {
		allowed[id] = struct{}{}
	}

	return &Handler{
		sender:       sender,
		allowedUsers: allowed,
		cooldown:     cooldown,
		deployer:     deployer,
		logger:       logger,
		auditLogger:  auditLogger,
	}
}

// IsAllowed returns true if the user is in the configured allowlist.
func (h *Handler) IsAllowed(userID int64) bool {
	_, ok := h.allowedUsers[userID]
	return ok
}

// HandleUpdate routes Telegram updates to the proper command handler.
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil || !update.Message.IsCommand() {
		return
	}

	switch update.Message.Command() {
	case "start":
		h.HandleStart(update.Message)
	case "deploy":
		h.HandleDeploy(ctx, update.Message)
	default:
		// Ignore unknown commands or reply with help
	}
}

// HandleStart responds to the /start command.
func (h *Handler) HandleStart(msg *tgbotapi.Message) {
	reply := "Привет! Я бот для развёртывания VPN.\nИспользуйте /deploy для запуска установки."
	h.sendPlain(msg.Chat.ID, reply)
}

// HandleDeploy processes the /deploy command with auth, cooldown, single-flight lock, and execution.
func (h *Handler) HandleDeploy(ctx context.Context, msg *tgbotapi.Message) {
	userID := msg.From.ID
	username := msg.From.UserName
	chatID := msg.Chat.ID

	// 1. Check Allowlist
	if !h.IsAllowed(userID) {
		h.auditLogger.Log(ctx, logging.AuditEvent{
			ActorID:  userID,
			Username: username,
			Action:   "deploy_vpn",
			Status:   "DENIED",
			Reason:   "not_in_allowlist",
		})
		h.sendPlain(chatID, "⛔ Нет доступа")
		return
	}

	// 2. Check Cooldown
	if val, ok := h.cooldowns.Load(userID); ok {
		lastDeploy := val.(time.Time)
		elapsed := time.Since(lastDeploy)
		if elapsed < h.cooldown {
			remaining := h.cooldown - elapsed
			mins := int(remaining.Minutes())
			secs := int(remaining.Seconds()) % 60
			h.auditLogger.Log(ctx, logging.AuditEvent{
				ActorID:  userID,
				Username: username,
				Action:   "deploy_vpn",
				Status:   "DENIED",
				Reason:   "cooldown_active",
			})
			h.sendPlain(chatID, fmt.Sprintf("⏱ Подождите ещё %d мин. %d сек. перед следующим запуском.", mins, secs))
			return
		}
	}

	// 3. Try Single-Flight Install Lock
	if !h.installLock.TryLock() {
		h.auditLogger.Log(ctx, logging.AuditEvent{
			ActorID:  userID,
			Username: username,
			Action:   "deploy_vpn",
			Status:   "DENIED",
			Reason:   "install_in_progress",
		})
		h.sendPlain(chatID, "⚠️ Установка уже выполняется, попробуйте позже.")
		return
	}
	defer h.installLock.Unlock()

	// 4. Send acknowledgment status message
	h.sendPlain(chatID, "⏳ Запускаю установку...")

	// 5. Run installation
	startTime := time.Now()
	uri, err := h.deployer.Deploy(ctx)
	duration := time.Since(startTime)

	// 6. Handle errors
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			h.auditLogger.Log(ctx, logging.AuditEvent{
				ActorID:  userID,
				Username: username,
				Action:   "deploy_vpn",
				Status:   "TIMEOUT",
				Reason:   "deadline_exceeded",
				Duration: duration,
			})
			h.sendPlain(chatID, "❌ Установка превысила лимит времени и была прервана.")
			return
		}

		h.auditLogger.Log(ctx, logging.AuditEvent{
			ActorID:  userID,
			Username: username,
			Action:   "deploy_vpn",
			Status:   "FAILED",
			Reason:   err.Error(),
			Duration: duration,
		})
		h.sendPlain(chatID, fmt.Sprintf("❌ Ошибка установки: %v", err))
		return
	}

	if uri == "" {
		h.auditLogger.Log(ctx, logging.AuditEvent{
			ActorID:  userID,
			Username: username,
			Action:   "deploy_vpn",
			Status:   "FAILED",
			Reason:   "empty_uri",
			Duration: duration,
		})
		h.sendPlain(chatID, "⚠️ Установка завершена, но URI не найден. Обратитесь к администратору.")
		return
	}

	// 7. Success
	h.cooldowns.Store(userID, time.Now())
	h.auditLogger.Log(ctx, logging.AuditEvent{
		ActorID:  userID,
		Username: username,
		Action:   "deploy_vpn",
		Status:   "SUCCESS",
		Duration: duration,
	})

	escapedURI := escapeMarkdownCode(uri)
	replyText := fmt.Sprintf("%s\n`%s`", escapeMarkdownV2Text("✅ Готово!"), escapedURI)
	h.sendMarkdownV2(chatID, replyText)
}

func (h *Handler) sendPlain(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.sender.Send(msg); err != nil {
		h.logger.Error("failed to send plain telegram message", "chat_id", chatID, "error", err)
	}
}

func (h *Handler) sendMarkdownV2(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	if _, err := h.sender.Send(msg); err != nil {
		h.logger.Error("failed to send markdown telegram message", "chat_id", chatID, "error", err)
	}
}

// escapeMarkdownV2Text escapes all MarkdownV2 special characters in text outside code/pre entities.
// See https://core.telegram.org/bots/api#markdownv2-style
func escapeMarkdownV2Text(s string) string {
	replacer := strings.NewReplacer(
		`_`, `\_`,
		`*`, `\*`,
		`[`, `\[`,
		`]`, `\]`,
		`(`, `\(`,
		`)`, `\)`,
		`~`, `\~`,
		"`", "\\`",
		`>`, `\>`,
		`#`, `\#`,
		`+`, `\+`,
		`-`, `\-`,
		`=`, `\=`,
		`|`, `\|`,
		`{`, `\{`,
		`}`, `\}`,
		`.`, `\.`,
		`!`, `\!`,
	)
	return replacer.Replace(s)
}

// escapeMarkdownCode escapes backticks and backslashes inside inline code in MarkdownV2.
func escapeMarkdownCode(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}
