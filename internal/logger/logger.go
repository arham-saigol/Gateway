package logger

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

var (
	urlRegex    = regexp.MustCompile(`https?://[^\s"'<>]+`)
	bearerRegex = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`)
	keyRegex    = regexp.MustCompile(`(?i)(?:key|token|secret|password|fw_|sk_|nov_)[a-zA-Z0-9_\-\.]+`)
)

func SanitizeError(msg string) string {
	msg = bearerRegex.ReplaceAllString(msg, "Bearer [REDACTED]")
	msg = urlRegex.ReplaceAllString(msg, "[REDACTED_URL]")
	msg = keyRegex.ReplaceAllString(msg, "[REDACTED_SECRET]")
	return msg
}

func sanitizeValue(key string, val any) any {
	lowerKey := strings.ToLower(key)
	if strings.Contains(lowerKey, "auth") ||
		strings.Contains(lowerKey, "secret") ||
		strings.Contains(lowerKey, "password") ||
		strings.Contains(lowerKey, "token") ||
		strings.Contains(lowerKey, "key") ||
		strings.Contains(lowerKey, "prompt") ||
		strings.Contains(lowerKey, "response") ||
		strings.Contains(lowerKey, "body") ||
		strings.Contains(lowerKey, "cookie") {
		return "[REDACTED]"
	}

	if s, ok := val.(string); ok {
		return SanitizeError(s)
	}
	return val
}

type RedactingHandler struct {
	inner slog.Handler
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	var newAttrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		newAttrs = append(newAttrs, slog.Attr{
			Key:   a.Key,
			Value: slog.AnyValue(sanitizeValue(a.Key, a.Value.Any())),
		})
		return true
	})

	sanitizedRecord := slog.NewRecord(r.Time, r.Level, SanitizeError(r.Message), r.PC)
	sanitizedRecord.AddAttrs(newAttrs...)
	return h.inner.Handle(ctx, sanitizedRecord)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var cleanAttrs []slog.Attr
	for _, a := range attrs {
		cleanAttrs = append(cleanAttrs, slog.Attr{
			Key:   a.Key,
			Value: slog.AnyValue(sanitizeValue(a.Key, a.Value.Any())),
		})
	}
	return &RedactingHandler{inner: h.inner.WithAttrs(cleanAttrs)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithGroup(name)}
}

func New(w io.Writer, level Level) *slog.Logger {
	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(&RedactingHandler{inner: jsonHandler})
}
