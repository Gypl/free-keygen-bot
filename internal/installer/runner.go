package installer

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
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

	shellCmd := fmt.Sprintf("curl -fsSL %s | bash", i.scriptURL)
	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %w", err)
	}

	// Use sync.Once to ensure PTY master is closed exactly once.
	// Closing the master guarantees the read goroutine unblocks (gets EIO/EBADF)
	// even if child processes still hold the PTY slave fd open.
	var ptmxCloseOnce sync.Once
	closePTY := func() {
		ptmxCloseOnce.Do(func() { _ = ptmx.Close() })
	}
	defer closePTY()

	// In Go 1.20+, ensure the entire process group is killed and PTY closed
	// if context is canceled while cmd is running or waiting.
	cmd.Cancel = func() error {
		killProcess(cmd)
		closePTY()
		return nil
	}

	var output bytes.Buffer
	var bufMu sync.Mutex
	outDone := make(chan struct{})

	go func() {
		defer close(outDone)
		buf := make([]byte, 1024)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				bufMu.Lock()
				output.Write(buf[:n])
				bufMu.Unlock()
			}
			if readErr != nil {
				break
			}
		}
	}()

	startOffset := 0

	// Execute each interaction step
	for idx, step := range seq.Steps {
		if ctx.Err() != nil {
			killProcess(cmd)
			closePTY()
			<-outDone
			return "", ctx.Err()
		}

		if step.WaitFor != nil {
			waitTimeout := step.WaitTimeout
			if waitTimeout <= 0 {
				waitTimeout = 30 * time.Second
			}
			newOffset, err := waitForPattern(ctx, &output, &bufMu, step.WaitFor, waitTimeout, startOffset)
			if err != nil {
				killProcess(cmd)
				closePTY()
				<-outDone
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
				killProcess(cmd)
				closePTY()
				<-outDone
				return "", ctx.Err()
			}
			bufMu.Lock()
			startOffset = output.Len()
			bufMu.Unlock()
		}

		// Write input + carriage return to terminal
		payload := []byte(step.Input + "\r")
		if _, err := ptmx.Write(payload); err != nil {
			killProcess(cmd)
			closePTY()
			<-outDone
			return "", fmt.Errorf("write step %d (%q): %w", idx, step.Input, err)
		}
	}

	// Wait for process completion or cancellation
	waitErr := cmd.Wait()
	closePTY() // Ensure read goroutine can exit even if child processes linger
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

func waitForPattern(ctx context.Context, buf *bytes.Buffer, mu *sync.Mutex, re *regexp.Regexp, timeout time.Duration, startOffset int) (int, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return startOffset, ctx.Err()
		case <-ticker.C:
			mu.Lock()
			var chunk []byte
			if buf.Len() > startOffset {
				chunk = buf.Bytes()[startOffset:]
			}
			matched := len(chunk) > 0 && re.Match(chunk)
			curLen := buf.Len()
			mu.Unlock()
			if matched {
				return curLen, nil
			}
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
