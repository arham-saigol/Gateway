package logger_test

import (
	"bytes"
	"strings"
	"testing"

	"arham-gateway/internal/logger"
)

func TestRedactedLoggerDoesNotLeakSecrets(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(&buf, logger.LevelInfo)

	secretToken := "sk-ant-api03-secret-key-123456789"
	promptBody := "Here is a secret user prompt text"

	l.Info("request received",
		"auth_header", "Bearer "+secretToken,
		"prompt", promptBody,
		"path", "/v1/chat/completions",
	)

	out := buf.String()
	if strings.Contains(out, secretToken) {
		t.Fatalf("logger leaked secret token in output: %s", out)
	}
	if strings.Contains(out, promptBody) {
		t.Fatalf("logger leaked prompt body in output: %s", out)
	}
	if !strings.Contains(out, "/v1/chat/completions") {
		t.Fatalf("logger omitted safe path: %s", out)
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	rawErr := "failed upstream call to https://api.fireworks.ai/v1/chat/completions with key fw_secret12345: 500 internal error"
	sanitized := logger.SanitizeError(rawErr)

	if strings.Contains(sanitized, "fw_secret12345") {
		t.Errorf("sanitized error leaked key: %s", sanitized)
	}
	if strings.Contains(sanitized, "https://api.fireworks.ai") {
		t.Errorf("sanitized error leaked upstream URL: %s", sanitized)
	}
}

func TestRedactedLoggerPreservesMetricsAndIdentifiers(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(&buf, logger.LevelInfo)

	l.Info("request completed",
		"input_tokens", 150,
		"output_tokens", 45,
		"cached_input_tokens", 10,
		"gateway_key_id", "gw-key-123",
		"model", "gpt-4o",
	)

	out := buf.String()
	if !strings.Contains(out, `"input_tokens":150`) {
		t.Errorf("expected input_tokens:150 in log, got: %s", out)
	}
	if !strings.Contains(out, `"output_tokens":45`) {
		t.Errorf("expected output_tokens:45 in log, got: %s", out)
	}
	if !strings.Contains(out, `"gateway_key_id":"gw-key-123"`) {
		t.Errorf("expected gateway_key_id:gw-key-123 in log, got: %s", out)
	}
}
