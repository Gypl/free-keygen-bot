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

// BuildSequence constructs the 8-step installer interaction sequence matching olcrtc install.sh.
func BuildSequence(commentPool []int, suffix string) Sequence {
	comment := randomComment(commentPool)
	if suffix == "" {
		suffix = "welcome"
	}

	delay := 1 * time.Second

	return Sequence{
		Steps: []Step{
			// 1. Mode: server (srv)
			{
				Input:       "1",
				WaitFor:     regexp.MustCompile(`(?i)(?:choice\s*\[1-2|select mode)`),
				WaitTimeout: 2 * time.Minute, // Allow time for initial package installation (git, podman)
				FixedDelay:  delay,
			},
			// 2. Provider: jitsi
			{
				Input:       "1",
				WaitFor:     regexp.MustCompile(`(?i)(?:choice\s*\[1-3|select provider)`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 3. Transport: datachannel
			{
				Input:       "1",
				WaitFor:     regexp.MustCompile(`(?i)(?:choice\s*\[1-4|select transport)`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 4. Jitsi server: choice from pool or manual URL fallback
			{
				Input:       comment,
				WaitFor:     regexp.MustCompile(`(?i)(?:jitsi server|by default:\s*1|enter the number|enter jitsi url)`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 5. Room options: auto-generate new room
			{
				Input:       "1",
				WaitFor:     regexp.MustCompile(`(?i)(?:room options|choice\s*\[1-2)`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 6. DNS server: accept default (8.8.8.8:53)
			{
				Input:       "",
				WaitFor:     regexp.MustCompile(`(?i)dns server`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 7. SOCKS5 proxy: no
			{
				Input:       "n",
				WaitFor:     regexp.MustCompile(`(?i)(?:socks5 proxy for egress|y/N)`),
				WaitTimeout: 30 * time.Second,
				FixedDelay:  delay,
			},
			// 8. Config comment: comment + suffix (wait for build and container launch)
			{
				Input:       comment + suffix,
				WaitFor:     regexp.MustCompile(`(?i)comment for the config`),
				WaitTimeout: 5 * time.Minute, // Podman pull, go build, and container run
				FixedDelay:  delay,
			},
		},
	}
}
