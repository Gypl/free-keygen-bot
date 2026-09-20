package logging

import (
	"log/slog"
	"regexp"
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

var (
	vpnURIPattern = regexp.MustCompile(`(?i)\b(olcrtc|vless|vmess|trojan|ss|ssr|hysteria|hysteria2|tuic|wireguard)://[^\s'"]+`)
	keyPattern    = regexp.MustCompile(`(?i)(key:?\s*)([0-9a-fA-F]{16,})`)
	tokenPattern  = regexp.MustCompile(`\b\d{8,12}:[a-zA-Z0-9_-]{30,50}\b`)
)

// MaskSensitive masks sensitive patterns (VPN URIs, private keys, bot tokens) in free-form text.
func MaskSensitive(s string) string {
	s = vpnURIPattern.ReplaceAllStringFunc(s, func(match string) string {
		idx := strings.Index(match, "://")
		if idx != -1 {
			return match[:idx+3] + "[REDACTED]"
		}
		return "[REDACTED_URI]"
	})
	s = keyPattern.ReplaceAllString(s, "${1}[REDACTED]")
	s = tokenPattern.ReplaceAllString(s, "[REDACTED]")
	return s
}

// SanitizeAttr is a slog.HandlerOptions.ReplaceAttr hook that masks sensitive keys and values.
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

	if a.Value.Kind() == slog.KindString {
		str := a.Value.String()
		masked := MaskSensitive(str)
		if masked != str {
			return slog.String(a.Key, masked)
		}
	}

	return a
}

