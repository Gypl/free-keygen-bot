package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

var (
	jitsiURLPromptRe = regexp.MustCompile(`(?i)enter jitsi url:`)
	fallbackJitsiURL = "https://meet.egovm.ru"
)

// Installer runs the automated installation inside a PTY session.
type Installer struct {
	scriptURL     string
	timeout       time.Duration
	commentPool   []int
	commentSuffix string
	logger        *slog.Logger
}

// New creates a new Installer instance.
func New(scriptURL string, timeout time.Duration, commentPool []int, commentSuffix string, logger *slog.Logger) *Installer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Installer{
		scriptURL:     scriptURL,
		timeout:       timeout,
		commentPool:   commentPool,
		commentSuffix: commentSuffix,
		logger:        logger,
	}
}

// Deploy executes the installer script, simulates user input, and extracts the VPN URI.
func (i *Installer) Deploy(ctx context.Context) (string, error) {
	if i.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, i.timeout)
		defer cancel()
	}

	seq := BuildSequence(i.commentPool, i.commentSuffix)

	// Remove any existing encryption key file to ensure each deployment gets a fresh, unique key
	shellCmd := fmt.Sprintf("rm -f \"$HOME/.olcrtc_key\" && curl -fsSL %q | bash", i.scriptURL)
	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)
	cmd.WaitDelay = 5 * time.Second

	var ptmx *os.File
	var ptmxCloseOnce sync.Once
	closePTY := func() {
		ptmxCloseOnce.Do(func() {
			if ptmx != nil {
				_ = ptmx.Close()
			}
		})
	}

	// Set cmd.Cancel BEFORE pty.Start (which calls cmd.Start internally)
	cmd.Cancel = func() error {
		killProcess(cmd)
		closePTY()
		return nil
	}

	var err error
	ptmx, err = pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %w", err)
	}

	var (
		waitOnce sync.Once
		waitErr  error
	)
	waitProcess := func() error {
		waitOnce.Do(func() {
			waitErr = cmd.Wait()
		})
		return waitErr
	}

	var output bytes.Buffer
	var bufMu sync.Mutex
	outDone := make(chan struct{})

	go func() {
		defer close(outDone)
		buf := make([]byte, 1024)
		var lineBuf bytes.Buffer
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				bufMu.Lock()
				output.Write(buf[:n])
				bufMu.Unlock()

				for _, b := range buf[:n] {
					if b == '\n' {
						raw := lineBuf.String()
						lineBuf.Reset()
						clean := strings.TrimSpace(ansiRe.ReplaceAllString(raw, ""))
						if clean != "" {
							i.logger.Info("terminal_output", "line", clean)
						}
					} else if b != '\r' {
						lineBuf.WriteByte(b)
					}
				}
			}
			if readErr != nil {
				if lineBuf.Len() > 0 {
					clean := strings.TrimSpace(ansiRe.ReplaceAllString(lineBuf.String(), ""))
					if clean != "" {
						i.logger.Info("terminal_output", "line", clean)
					}
				}
				break
			}
		}
	}()

	// Defer comprehensive cleanup so that process reaping and PTY closing always occur,
	// preventing zombie (defunct) processes on any error or early return.
	cleanup := func() {
		killProcess(cmd)
		closePTY()
		<-outDone
		_ = waitProcess()
	}
	defer cleanup()

	// Proactively kill process group and close PTY if context is canceled
	go func() {
		select {
		case <-ctx.Done():
			killProcess(cmd)
			closePTY()
		case <-outDone:
		}
	}()

	startOffset := 0

	// Execute each interaction step
	for idx, step := range seq.Steps {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		prevOffset := startOffset
		if step.WaitFor != nil {
			waitTimeout := step.WaitTimeout
			if waitTimeout <= 0 {
				waitTimeout = 30 * time.Second
			}
			newOffset, err := waitForPattern(ctx, outDone, ptmx, &output, &bufMu, step.WaitFor, waitTimeout, startOffset)
			if err != nil {
				bufMu.Lock()
				tail := tailLines(output.String(), 20)
				bufMu.Unlock()
				i.logger.Warn("step wait pattern timed out", "step", idx, "input", step.Input, "tail", tail)
				return "", fmt.Errorf("step %d (%q): %w", idx, step.Input, err)
			}
			startOffset = newOffset
			time.Sleep(50 * time.Millisecond)
		} else if step.FixedDelay > 0 {
			select {
			case <-time.After(step.FixedDelay):
			case <-ctx.Done():
				return "", ctx.Err()
			case <-outDone:
				return "", errors.New("process terminated unexpectedly during delay")
			}
			bufMu.Lock()
			startOffset = output.Len()
			bufMu.Unlock()
		}

		// Determine input: if this step matched a manual Jitsi URL prompt,
		// provide a fallback Jitsi URL rather than a server index number.
		input := step.Input
		inputDesc := step.InputDesc
		var promptText string
		bufMu.Lock()
		if prevOffset < output.Len() {
			chunk := ansiRe.ReplaceAllString(output.String()[prevOffset:startOffset], "")
			lines := strings.Split(strings.TrimSpace(chunk), "\n")
			if len(lines) > 0 {
				promptText = strings.TrimSpace(lines[len(lines)-1])
			}
			if jitsiURLPromptRe.Match(output.Bytes()[prevOffset:]) {
				input = fallbackJitsiURL
				inputDesc = "fallback Jitsi URL: " + fallbackJitsiURL
			}
		}
		bufMu.Unlock()

		// Pause briefly before sending input so interactive prompts and PTY are fully ready
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		case <-outDone:
			return "", errors.New("process terminated unexpectedly before input")
		}

		i.logger.Info("installer_menu_choice",
			"step", idx+1,
			"total", len(seq.Steps),
			"menu", step.Name,
			"prompt", promptText,
			"answer", input,
			"description", inputDesc,
		)

		// Write input + carriage return to terminal
		payload := []byte(input + "\r")
		if _, err := ptmx.Write(payload); err != nil {
			return "", fmt.Errorf("write step %d (%q): %w", idx, input, err)
		}

		// Pause briefly after sending input to allow the shell to process and echo
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		case <-outDone:
		}
	}

	i.logger.Info("all installer inputs sent, waiting for script to finish...")

	// Wait for process completion or cancellation
	_ = waitProcess()
	closePTY()
	<-outDone

	bufMu.Lock()
	fullOutput := output.String()
	bufMu.Unlock()

	if ctx.Err() != nil {
		i.logger.Warn("installer context deadline exceeded", "error", ctx.Err())
		return "", ctx.Err()
	}

	if waitErr != nil {
		tail := tailLines(fullOutput, 30)
		i.logger.Warn("installer process exited with error", "error", waitErr, "tail", tail)
		return "", fmt.Errorf("installer failed: %w", waitErr)
	}

	uri, err := ExtractURI(fullOutput)
	if err != nil {
		tail := tailLines(fullOutput, 20)
		i.logger.Warn("installer finished but uri extraction failed", "tail", tail)
		return "", fmt.Errorf("uri extraction: %w", err)
	}

	return uri, nil
}

// killProcess kills the process group and then the leader process.
func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	killProcessGroup(cmd) // Platform-specific: kill entire process group
	_ = cmd.Process.Kill()
}

func waitForPattern(
	ctx context.Context,
	outDone <-chan struct{},
	ptmx *os.File,
	buf *bytes.Buffer,
	mu *sync.Mutex,
	re *regexp.Regexp,
	timeout time.Duration,
	startOffset int,
) (int, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		mu.Lock()
		var chunk []byte
		if buf.Len() > startOffset {
			chunk = buf.Bytes()[startOffset:]
		}
		matched := len(chunk) > 0 && re.Match(chunk)
		curLen := buf.Len()

		// Check if installer unexpectedly prompted for a manual Jitsi URL while we were waiting for another pattern (e.g. Room options)
		isJitsiURLPrompt := ptmx != nil && len(chunk) > 0 && jitsiURLPromptRe.Match(chunk) && !re.Match(chunk)
		mu.Unlock()

		if matched {
			return curLen, nil
		}

		if isJitsiURLPrompt {
			// Auto-respond with fallback Jitsi URL and advance offset past this prompt
			if _, err := ptmx.Write([]byte(fallbackJitsiURL + "\r")); err == nil {
				time.Sleep(50 * time.Millisecond)
				startOffset = curLen
				continue
			}
		}

		select {
		case <-ctx.Done():
			return startOffset, ctx.Err()
		case <-outDone:
			// Process terminated early, check buffer one last time
			mu.Lock()
			if buf.Len() > startOffset {
				chunk = buf.Bytes()[startOffset:]
			}
			matched = len(chunk) > 0 && re.Match(chunk)
			curLen = buf.Len()
			mu.Unlock()
			if matched {
				return curLen, nil
			}
			return curLen, fmt.Errorf("process terminated unexpectedly while waiting for pattern %s", re.String())
		case <-ticker.C:
			if time.Now().After(deadline) {
				return curLen, fmt.Errorf("timeout waiting for pattern %s", re.String())
			}
		}
	}
}

func tailLines(s string, count int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= count {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-count:], "\n")
}
