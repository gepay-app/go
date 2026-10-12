package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		raw, env string
		want     slog.Level
	}{
		{"", "production", slog.LevelInfo},
		{"", "development", slog.LevelDebug},
		{"warn", "development", slog.LevelWarn},
		{"ERROR", "development", slog.LevelError},
		{"bogus", "production", slog.LevelInfo}, // config rejects this; fallback must stay safe
	}
	for _, tt := range tests {
		if got := parseLevel(tt.raw, tt.env); got != tt.want {
			t.Errorf("parseLevel(%q,%q)=%v want %v", tt.raw, tt.env, got, tt.want)
		}
	}
}

func TestFormat(t *testing.T) {
	if got := format("", "production"); got != "json" {
		t.Errorf("production default = %q, want json", got)
	}
	if got := format("", "development"); got != "text" {
		t.Errorf("development default = %q, want text", got)
	}
	if got := format("JSON", "development"); got != "json" {
		t.Errorf("explicit = %q, want json", got)
	}
}

func TestRequestID(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Fatalf("empty ctx = %q, want empty", got)
	}
	if got := RequestID(WithRequestID(context.Background(), "x")); got != "x" {
		t.Fatalf("got %q, want x", got)
	}
}

func TestFromContext(t *testing.T) {
	if got := FromContext(context.Background()); got != slog.Default() {
		t.Fatal("without a stored logger, FromContext should return slog.Default()")
	}

	var buf bytes.Buffer
	reqLog := slog.New(slog.NewTextHandler(&buf, nil)).With("request_id", "req-123")
	ctx := WithLogger(WithRequestID(context.Background(), "req-123"), reqLog)

	if got := FromContext(ctx); got != reqLog {
		t.Fatal("FromContext did not return the stored request logger")
	}
	FromContext(ctx).Info("hello")
	if !strings.Contains(buf.String(), "request_id=req-123") {
		t.Fatalf("request_id missing from %q", buf.String())
	}
}
