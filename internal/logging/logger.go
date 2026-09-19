package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a new structured slog.Logger configured with level, JSON/text handler,
// and sensitive field sanitization.
func NewLogger(levelStr string, isJSON bool) *slog.Logger {
	return NewLoggerWithWriter(os.Stdout, levelStr, isJSON)
}

// NewLoggerWithWriter creates a logger outputting to the specified io.Writer.
func NewLoggerWithWriter(w io.Writer, levelStr string, isJSON bool) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		AddSource:   level == slog.LevelDebug,
		ReplaceAttr: SanitizeAttr,
	}

	var handler slog.Handler
	if isJSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}
