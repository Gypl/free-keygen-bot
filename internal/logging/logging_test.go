package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMasking(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "info", true)

	token := SensitiveToken("secret_token_12345")
	uri := VPNURI("vless://user@1.2.3.4:443?type=tcp#my-node")

	logger.Info("testing credentials",
		"bot_token", token,
		"config_uri", uri,
		"plain_secret", "super_secret_value",
		"plain_uri", "trojan://pass@5.6.7.8:443",
	)

	out := buf.String()

	if strings.Contains(out, "secret_token_12345") {
		t.Errorf("token leaked in output: %s", out)
	}
	if strings.Contains(out, "my-node") {
		t.Errorf("uri details leaked in output: %s", out)
	}
	if strings.Contains(out, "super_secret_value") {
		t.Errorf("plain_secret leaked in output: %s", out)
	}
	if strings.Contains(out, "pass@5.6.7.8") {
		t.Errorf("plain_uri leaked in output: %s", out)
	}

	if !strings.Contains(out, "vless://[REDACTED]") {
		t.Errorf("expected vless://[REDACTED], got %s", out)
	}
	if !strings.Contains(out, "trojan://[REDACTED]") {
		t.Errorf("expected trojan://[REDACTED], got %s", out)
	}
}

func TestAuditLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "info", true)
	audit := NewAuditLogger(logger)

	audit.Log(context.Background(), AuditEvent{
		ActorID:  123456789,
		Username: "testuser",
		Action:   "deploy_vpn",
		Status:   "SUCCESS",
		Duration: 1500 * time.Millisecond,
	})

	out := buf.String()

	var record map[string]any
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("failed to parse json log: %v (raw: %s)", err, out)
	}

	if record["log_type"] != "audit" {
		t.Errorf("expected log_type=audit, got %v", record["log_type"])
	}

	auditData, ok := record["audit"].(map[string]any)
	if !ok {
		t.Fatalf("audit group missing in record: %v", record)
	}

	if int64(auditData["actor_id"].(float64)) != 123456789 {
		t.Errorf("expected actor_id 123456789, got %v", auditData["actor_id"])
	}
	if auditData["status"] != "SUCCESS" {
		t.Errorf("expected status SUCCESS, got %v", auditData["status"])
	}
}
