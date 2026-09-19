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

	if len(seq.Steps) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(seq.Steps))
	}

	// Step 0: "1"
	if seq.Steps[0].Input != "1" {
		t.Errorf("step 0: expected '1', got %q", seq.Steps[0].Input)
	}

	// Step 1: "1"
	if seq.Steps[1].Input != "1" {
		t.Errorf("step 1: expected '1', got %q", seq.Steps[1].Input)
	}

	// Step 2: random comment from pool
	commentStr := seq.Steps[2].Input
	commentNum, err := strconv.Atoi(commentStr)
	if err != nil {
		t.Fatalf("step 2: comment %q is not a number: %v", commentStr, err)
	}
	found := false
	for _, p := range pool {
		if p == commentNum {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("step 2: comment %d not in pool %v", commentNum, pool)
	}

	// Step 3: "1"
	if seq.Steps[3].Input != "1" {
		t.Errorf("step 3: expected '1', got %q", seq.Steps[3].Input)
	}

	// Step 4: "" (Enter)
	if seq.Steps[4].Input != "" {
		t.Errorf("step 4: expected empty Enter, got %q", seq.Steps[4].Input)
	}

	// Step 5: "n"
	if seq.Steps[5].Input != "n" {
		t.Errorf("step 5: expected 'n', got %q", seq.Steps[5].Input)
	}

	// Step 6: comment + suffix
	expectedFinal := commentStr + suffix
	if seq.Steps[6].Input != expectedFinal {
		t.Errorf("step 6: expected %q, got %q", expectedFinal, seq.Steps[6].Input)
	}
	if !strings.HasSuffix(seq.Steps[6].Input, suffix) {
		t.Errorf("step 6: expected suffix %q, got %q", suffix, seq.Steps[6].Input)
	}
}
