package installer

import (
	"strconv"
	"strings"
	"testing"
)

func TestBuildSequence(t *testing.T) {
	pool := []int{1, 4, 5, 6, 7, 9, 10, 11, 12, 14, 15}
	suffix := "welcome"

	seq := BuildSequence(pool, suffix)

	if len(seq.Steps) != 8 {
		t.Fatalf("expected 8 steps, got %d", len(seq.Steps))
	}

	// Step 0: "1" (mode: srv)
	if seq.Steps[0].Input != "1" {
		t.Errorf("step 0: expected '1', got %q", seq.Steps[0].Input)
	}

	// Step 1: "1" (provider: jitsi)
	if seq.Steps[1].Input != "1" {
		t.Errorf("step 1: expected '1', got %q", seq.Steps[1].Input)
	}

	// Step 2: "1" (transport: datachannel)
	if seq.Steps[2].Input != "1" {
		t.Errorf("step 2: expected '1', got %q", seq.Steps[2].Input)
	}

	// Step 3: random comment from pool (jitsi server choice)
	commentStr := seq.Steps[3].Input
	commentNum, err := strconv.Atoi(commentStr)
	if err != nil {
		t.Fatalf("step 3: comment %q is not a number: %v", commentStr, err)
	}
	found := false
	for _, p := range pool {
		if p == commentNum {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("step 3: comment %d not in pool %v", commentNum, pool)
	}

	// Step 4: "1" (room: auto-generate)
	if seq.Steps[4].Input != "1" {
		t.Errorf("step 4: expected '1', got %q", seq.Steps[4].Input)
	}

	// Step 5: "" (dns: default)
	if seq.Steps[5].Input != "" {
		t.Errorf("step 5: expected empty Enter, got %q", seq.Steps[5].Input)
	}

	// Step 6: "n" (socks5: no)
	if seq.Steps[6].Input != "n" {
		t.Errorf("step 6: expected 'n', got %q", seq.Steps[6].Input)
	}

	// Step 7: comment + suffix (config comment)
	expectedFinal := commentStr + suffix
	if seq.Steps[7].Input != expectedFinal {
		t.Errorf("step 7: expected %q, got %q", expectedFinal, seq.Steps[7].Input)
	}
	if !strings.HasSuffix(seq.Steps[7].Input, suffix) {
		t.Errorf("step 7: expected suffix %q, got %q", suffix, seq.Steps[7].Input)
	}

	// Verify all steps have a non-nil WaitFor pattern, non-empty Name and InputDesc
	for idx, step := range seq.Steps {
		if step.WaitFor == nil {
			t.Errorf("step %d has nil WaitFor pattern", idx)
		}
		if step.Name == "" {
			t.Errorf("step %d has empty Name", idx)
		}
		if step.InputDesc == "" {
			t.Errorf("step %d has empty InputDesc", idx)
		}
	}
}
