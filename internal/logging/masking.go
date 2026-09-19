package logging

import (
	"log/slog"
	"strings"
)

// SensitiveToken masks authentication tokens in log output
type SensitiveToken string

func (t SensitiveToken) LogValue() slog.Value {
	if t == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[REDACTED]")
}

func (t SensitiveToken) String() string {
	return string(t)
}

// VPNURI masks VPN credentials while preserving the protocol scheme for diagnostics
type VPNURI string

func (u VPNURI) LogValue() slog.Value {
	str := string(u)
	if str == "" {
		return slog.StringValue("")
	}
	if idx := strings.Index(str, "://"); idx != -1 {
		scheme := str[:idx]
		return slog.StringValue(scheme + "://[REDACTED]")
	}
	return slog.StringValue("[REDACTED_URI]")
}

func (u VPNURI) String() string {
	return string(u)
}

// SanitizeAttr is a slog.HandlerOptions.ReplaceAttr hook that masks sensitive keys
func SanitizeAttr(groups []string, a slog.Attr) slog.Attr {
	// Standardize time formatting if desired
	if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		return slog.String(a.Key, a.Value.Time().UTC().Format("2006-01-02T15:04:05.000Z07:00"))
	}

	key := strings.ToLower(a.Key)
	if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "password") || strings.Contains(key, "auth") {
		return slog.String(a.Key, "[REDACTED]")
	}

	if strings.Contains(key, "uri") {
		if str, ok := a.Value.Any().(string); ok && str != "" {
			if idx := strings.Index(str, "://"); idx != -1 {
				return slog.String(a.Key, str[:idx]+"://[REDACTED]")
			}
			return slog.String(a.Key, "[REDACTED_URI]")
		}
	}

	return a
}
