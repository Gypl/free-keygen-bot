package installer

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWaitForPattern_Success(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	outDone := make(chan struct{})
	defer close(outDone)

	buf.WriteString("some initial output\nSelect mode:\n")

	re := regexp.MustCompile(`Select mode:`)
	offset, err := waitForPattern(context.Background(), outDone, nil, &buf, &mu, re, 1*time.Second, 0)
	if err != nil {
		t.Fatalf("expected pattern to match, got error: %v", err)
	}

	if offset != buf.Len() {
		t.Errorf("expected offset %d, got %d", buf.Len(), offset)
	}
}

func TestWaitForPattern_EarlyProcessExit(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	outDone := make(chan struct{})

	// Simulate process exiting immediately
	close(outDone)

	re := regexp.MustCompile(`Select mode:`)
	start := time.Now()
	_, err := waitForPattern(context.Background(), outDone, nil, &buf, &mu, re, 10*time.Second, 0)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on early process exit, got nil")
	}

	if !strings.Contains(err.Error(), "process terminated unexpectedly") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Ensure it didn't wait the full 10 seconds
	if elapsed > 2*time.Second {
		t.Errorf("waitForPattern took too long to detect process exit: %v", elapsed)
	}
}

func TestWaitForPattern_Timeout(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	outDone := make(chan struct{})
	defer close(outDone)

	re := regexp.MustCompile(`Non-existent pattern`)
	_, err := waitForPattern(context.Background(), outDone, nil, &buf, &mu, re, 100*time.Millisecond, 0)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout waiting for pattern") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTailLines(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	tail := tailLines(input, 3)
	expected := "line3\nline4\nline5"
	if tail != expected {
		t.Errorf("expected %q, got %q", expected, tail)
	}

	shortInput := "line1\nline2"
	tailShort := tailLines(shortInput, 5)
	if tailShort != shortInput {
		t.Errorf("expected %q, got %q", shortInput, tailShort)
	}
}
