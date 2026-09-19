package installer

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	uriRe  = regexp.MustCompile(`(?m)^uri:\s*(\S+)\s*$`)
)

// ExtractURI searches the installer output for the last "uri: <link>" line and returns the URI.
func ExtractURI(output string) (string, error) {
	// Strip ANSI escape codes
	cleaned := ansiRe.ReplaceAllString(output, "")
	// Normalize Windows / CRLF newlines
	cleaned = strings.ReplaceAll(cleaned, "\r", "")

	matches := uriRe.FindAllStringSubmatch(cleaned, -1)
	if len(matches) == 0 {
		return "", errors.New("uri not found in installer output")
	}

	last := matches[len(matches)-1]
	uri := strings.TrimSpace(last[1])
	if uri == "" {
		return "", errors.New("extracted uri is empty")
	}

	return uri, nil
}
