package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-deploy-bot/internal/logging"
)

// mockSender records sent messages.
type mockSender struct {
	mu       sync.Mutex
	messages []tgbotapi.MessageConfig
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		m.messages = append(m.messages, msg)
	}
	return tgbotapi.Message{}, nil
}

func (m *mockSender) getMessages() []tgbotapi.MessageConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]tgbotapi.MessageConfig, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func (m *mockSender) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

// mockDeployer simulates the VPN installer.
type mockDeployer struct {
	deployFunc func(ctx context.Context) (string, error)
	callCount  int64
}

func (m *mockDeployer) Deploy(ctx context.Context) (string, error) {
	atomic.AddInt64(&m.callCount, 1)
	if m.deployFunc != nil {
		return m.deployFunc(ctx)
	}
	return "vless://mock-uuid@1.2.3.4:443#node", nil
}

func newTestHandler(sender MessageSender, allowed []int64, cooldown time.Duration, deployer Deployer) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	audit := logging.NewAuditLogger(logger)
	return NewHandler(sender, allowed, cooldown, deployer, logger, audit)
}

func makeMessage(userID int64, username string, chatID int64, cmd string) *tgbotapi.Message {
	return &tgbotapi.Message{
		From: &tgbotapi.User{
			ID:       userID,
			UserName: username,
		},
		Chat: &tgbotapi.Chat{
			ID:   chatID,
			Type: "private",
		},
		Text: "/" + cmd,
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/" + cmd)},
		},
	}
}

// --- User Story 1 Tests ---

func TestHandleStart(t *testing.T) {
	sender := &mockSender{}
	h := newTestHandler(sender, []int64{100}, 10*time.Minute, &mockDeployer{})

	msg := makeMessage(100, "alice", 100, "start")
	h.HandleStart(msg)

	msgs := sender.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Text, "Привет! Я бот для развёртывания VPN") {
		t.Errorf("unexpected start response: %s", msgs[0].Text)
	}
}

func TestHandleDeploy_Success(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			return "vless://test-uuid@1.2.3.4:443#alice-node", nil
		},
	}
	h := newTestHandler(sender, []int64{100}, 10*time.Minute, deployer)

	msg := makeMessage(100, "alice", 100, "deploy")
	h.HandleDeploy(context.Background(), msg)

	if atomic.LoadInt64(&deployer.callCount) != 1 {
		t.Errorf("expected deployer to be called once, got %d", deployer.callCount)
	}

	msgs := sender.getMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (status + result), got %d", len(msgs))
	}

	if !strings.Contains(msgs[0].Text, "Запускаю установку") {
		t.Errorf("expected status message, got: %s", msgs[0].Text)
	}

	if !strings.Contains(msgs[1].Text, "vless://test-uuid@1.2.3.4:443#alice-node") {
		t.Errorf("expected URI in final message, got: %s", msgs[1].Text)
	}
	// Verify MarkdownV2 special chars are escaped in the text part
	if !strings.Contains(msgs[1].Text, `\!`) {
		t.Errorf("expected '!' to be escaped as '\\!' in MarkdownV2, got: %s", msgs[1].Text)
	}
	if msgs[1].ParseMode != tgbotapi.ModeMarkdownV2 {
		t.Errorf("expected MarkdownV2 parse mode, got %s", msgs[1].ParseMode)
	}
}

// --- User Story 2 Tests: Allowlist & Auth ---

func TestHandleDeploy_UnauthorizedUser(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{}
	h := newTestHandler(sender, []int64{100, 200}, 10*time.Minute, deployer)

	// User 999 is not in allowlist
	msg := makeMessage(999, "mallory", 999, "deploy")
	h.HandleDeploy(context.Background(), msg)

	if atomic.LoadInt64(&deployer.callCount) != 0 {
		t.Errorf("deployer should NOT be called for unauthorized user")
	}

	msgs := sender.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Text, "Нет доступа") {
		t.Errorf("expected 'Нет доступа', got: %s", msgs[0].Text)
	}
}

func TestIsAllowed(t *testing.T) {
	h := newTestHandler(&mockSender{}, []int64{100, 200}, 0, &mockDeployer{})

	if !h.IsAllowed(100) {
		t.Errorf("expected 100 to be allowed")
	}
	if !h.IsAllowed(200) {
		t.Errorf("expected 200 to be allowed")
	}
	if h.IsAllowed(300) {
		t.Errorf("expected 300 to NOT be allowed")
	}
}

// --- User Story 3 Tests: Concurrency & Single-Flight Lock ---

func TestHandleDeploy_ConcurrentExecution(t *testing.T) {
	sender := &mockSender{}
	started := make(chan struct{})
	block := make(chan struct{})

	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			close(started)
			<-block
			return "vless://result", nil
		},
	}
	h := newTestHandler(sender, []int64{100, 200}, 0, deployer)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		h.HandleDeploy(context.Background(), makeMessage(100, "user1", 100, "deploy"))
	}()

	// Wait until user 1 acquires lock and enters Deploy
	<-started

	// User 2 attempts deploy while user 1 is in progress
	h.HandleDeploy(context.Background(), makeMessage(200, "user2", 200, "deploy"))

	// Unblock user 1
	close(block)
	wg.Wait()

	// Deployer should have been called only once
	if atomic.LoadInt64(&deployer.callCount) != 1 {
		t.Errorf("expected exactly 1 deploy call, got %d", deployer.callCount)
	}

	// Verify user 2 received the "already in progress" warning
	msgs := sender.getMessages()
	foundBusyWarning := false
	for _, m := range msgs {
		if m.ChatID == 200 && strings.Contains(m.Text, "Установка уже выполняется") {
			foundBusyWarning = true
			break
		}
	}
	if !foundBusyWarning {
		t.Errorf("user 2 did not receive install in progress warning")
	}
}

// --- User Story 4 Tests: Rate Limiting & Cooldown ---

func TestHandleDeploy_Cooldown(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{}
	cooldown := 10 * time.Minute
	h := newTestHandler(sender, []int64{100, 200}, cooldown, deployer)

	// First deploy from user 100
	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))
	if atomic.LoadInt64(&deployer.callCount) != 1 {
		t.Fatalf("expected 1 deploy call, got %d", deployer.callCount)
	}

	sender.clear()

	// Immediate second deploy from user 100 -> rejected due to cooldown
	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))
	if atomic.LoadInt64(&deployer.callCount) != 1 {
		t.Errorf("deployer should NOT have been called during cooldown, count: %d", deployer.callCount)
	}
	msgs := sender.getMessages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "Подождите ещё") {
		t.Errorf("expected cooldown warning, got: %v", msgs)
	}

	sender.clear()

	// Another authorized user (200) can still deploy (cooldown is per-user)
	h.HandleDeploy(context.Background(), makeMessage(200, "bob", 200, "deploy"))
	if atomic.LoadInt64(&deployer.callCount) != 2 {
		t.Errorf("user 200 should be allowed to deploy, count: %d", deployer.callCount)
	}
}

// --- User Story 5 Tests: Error Handling ---

func TestHandleDeploy_DeadlineExceeded(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			return "", context.DeadlineExceeded
		},
	}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))

	msgs := sender.getMessages()
	foundTimeoutMsg := false
	for _, m := range msgs {
		if strings.Contains(m.Text, "превысила лимит времени") {
			foundTimeoutMsg = true
			break
		}
	}
	if !foundTimeoutMsg {
		t.Errorf("expected timeout message, got: %v", msgs)
	}

	// Verify lock was released: another call should proceed to deployer
	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))
	if atomic.LoadInt64(&deployer.callCount) != 2 {
		t.Errorf("lock was not released after timeout")
	}
}

func TestHandleDeploy_ProcessError(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			return "", errors.New("network failure: curl 404")
		},
	}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))

	msgs := sender.getMessages()
	foundErrMsg := false
	for _, m := range msgs {
		if strings.Contains(m.Text, "Ошибка установки") && strings.Contains(m.Text, "network failure") {
			foundErrMsg = true
			break
		}
	}
	if !foundErrMsg {
		t.Errorf("expected error message with details, got: %v", msgs)
	}

	// Lock released
	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))
	if atomic.LoadInt64(&deployer.callCount) != 2 {
		t.Errorf("lock was not released after process error")
	}
}

func TestHandleDeploy_EmptyURI(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			return "", nil // Succeeded with empty URI
		},
	}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	h.HandleDeploy(context.Background(), makeMessage(100, "alice", 100, "deploy"))

	msgs := sender.getMessages()
	foundWarning := false
	for _, m := range msgs {
		if strings.Contains(m.Text, "URI не найден") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected 'URI не найден' warning, got: %v", msgs)
	}
}

func TestHandleUpdate_NonCommandIgnored(t *testing.T) {
	sender := &mockSender{}
	h := newTestHandler(sender, []int64{100}, 0, &mockDeployer{})

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 100},
			Chat: &tgbotapi.Chat{ID: 100},
			Text: "hello bot",
		},
	}

	h.HandleUpdate(context.Background(), update)
	if len(sender.getMessages()) != 0 {
		t.Errorf("expected non-command message to be ignored")
	}
}

func TestEscapeMarkdownCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "vless://test", expected: "vless://test"},
		{input: "vless://" + "`" + "test" + "`", expected: "vless://" + "\\`" + "test" + "\\`"},
		{input: `path\to\node`, expected: `path\\to\\node`},
	}
	for _, tt := range tests {
		got := escapeMarkdownCode(tt.input)
		if got != tt.expected {
			t.Errorf("escapeMarkdownCode(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestEscapeMarkdownV2Text(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "✅ Готово!", expected: `✅ Готово\!`},
		{input: "hello", expected: "hello"},
		{input: "a.b!c", expected: `a\.b\!c`},
		{input: "test_bold*italic", expected: `test\_bold\*italic`},
		{input: "[link](url)", expected: `\[link\]\(url\)`},
		{input: "a+b=c", expected: `a\+b\=c`},
		{input: `test\slash`, expected: `test\\slash`},
	}
	for _, tt := range tests {
		got := escapeMarkdownV2Text(tt.input)
		if got != tt.expected {
			t.Errorf("escapeMarkdownV2Text(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestHandleUpdate_NilSender(t *testing.T) {
	sender := &mockSender{}
	h := newTestHandler(sender, []int64{100}, 0, &mockDeployer{})

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: nil,
			Chat: &tgbotapi.Chat{ID: 100, Type: "private"},
			Text: "/deploy",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 7},
			},
		},
	}

	// Should not panic
	h.HandleUpdate(context.Background(), update)
	if len(sender.getMessages()) != 0 {
		t.Errorf("expected no messages when sender is nil")
	}
}

func TestHandleUpdate_NonPrivateChat(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 100, UserName: "alice"},
			Chat: &tgbotapi.Chat{ID: -100123456, Type: "supergroup"},
			Text: "/deploy",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 7},
			},
		},
	}

	h.HandleUpdate(context.Background(), update)
	if atomic.LoadInt64(&deployer.callCount) != 0 {
		t.Errorf("deployer should not be called in non-private chat")
	}

	msgs := sender.getMessages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "только в личных сообщениях") {
		t.Errorf("expected private chat warning, got: %v", msgs)
	}
}

func TestHandleDeploy_WithArguments(t *testing.T) {
	sender := &mockSender{}
	deployer := &mockDeployer{}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	msg := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 100, UserName: "alice"},
		Chat: &tgbotapi.Chat{ID: 100, Type: "private"},
		Text: "/deploy unexpected_arg",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 7},
		},
	}

	h.HandleDeploy(context.Background(), msg)
	if atomic.LoadInt64(&deployer.callCount) != 0 {
		t.Errorf("deployer should not be called when arguments are provided")
	}

	msgs := sender.getMessages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "не принимает параметров") {
		t.Errorf("expected no-args warning, got: %v", msgs)
	}
}

type failMarkdownSender struct {
	mockSender
}

func (f *failMarkdownSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		if msg.ParseMode == tgbotapi.ModeMarkdownV2 {
			return tgbotapi.Message{}, errors.New("Bad Request: can't parse entities")
		}
	}
	return f.mockSender.Send(c)
}

func TestHandleDeploy_MarkdownFallback(t *testing.T) {
	sender := &failMarkdownSender{}
	deployer := &mockDeployer{
		deployFunc: func(ctx context.Context) (string, error) {
			return "olcrtc://test-uri", nil
		},
	}
	h := newTestHandler(sender, []int64{100}, 0, deployer)

	msg := makeMessage(100, "alice", 100, "deploy")
	h.HandleDeploy(context.Background(), msg)

	msgs := sender.getMessages()
	// Should have sent: 1) status "Запускаю установку...", 2) fallback plain text "✅ Готово!\nolcrtc://test-uri"
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ParseMode != "" {
		t.Errorf("expected plain text fallback, got parse mode: %s", msgs[1].ParseMode)
	}
	if !strings.Contains(msgs[1].Text, "olcrtc://test-uri") {
		t.Errorf("expected fallback message to contain URI, got: %s", msgs[1].Text)
	}
}

