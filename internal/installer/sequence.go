package installer

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strconv"
	"time"
)

// Step represents a single interaction step in the console installer session.
type Step struct {
	Input       string         // Text to send to PTY (carriage return is appended automatically)
	WaitFor     *regexp.Regexp // Optional: regexp pattern to wait for in output before sending input
	WaitTimeout time.Duration  // Max duration to wait for WaitFor pattern
	FixedDelay  time.Duration  // Fallback delay before sending input if WaitFor is not set
}

// Sequence is an ordered series of interaction steps.
type Sequence struct {
	Steps []Step
}

// randomComment picks a random value from pool using cryptographically secure random.
func randomComment(pool []int) string {
	if len(pool) == 0 {
		return "1"
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
	if err != nil {
		return strconv.Itoa(pool[0])
	}
	return strconv.Itoa(pool[n.Int64()])
}

// BuildSequence constructs the 7-step installer interaction sequence.
func BuildSequence(commentPool []int, suffix string) Sequence {
	comment := randomComment(commentPool)
	if suffix == "" {
		suffix = "welcome"
	}

	delay := 1 * time.Second

	return Sequence{
		Steps: []Step{
			{Input: "1", FixedDelay: delay},
			{Input: "1", FixedDelay: delay},
			{Input: comment, FixedDelay: delay},
			{Input: "1", FixedDelay: delay},
			{Input: "", FixedDelay: delay}, // Empty Enter
			{Input: "n", FixedDelay: delay},
			{Input: comment + suffix, FixedDelay: delay},
		},
	}
}
