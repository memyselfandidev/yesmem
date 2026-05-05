package proxy

import (
	"testing"
	"time"
)

func TestExtractClaudeCodeVersion(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "claude code token",
			userAgent: "claude-code/2.1.117 node/v20",
			want:      "2.1.117",
		},
		{
			name:      "anthropic package token",
			userAgent: "@anthropic-ai/claude-code/2.2.0 bun/1.2.3",
			want:      "2.2.0",
		},
		{
			name:      "claude cli token",
			userAgent: "Claude-CLI/1.0.3",
			want:      "1.0.3",
		},
		{
			name:      "unknown",
			userAgent: "curl/8.0",
			want:      "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractClaudeCodeVersion(tt.userAgent); got != tt.want {
				t.Fatalf("extractClaudeCodeVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldLogRewriteMissDedupesByFunctionAndVersion(t *testing.T) {
	s := &Server{}
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	version, ok := s.shouldLogRewriteMiss("StripToneBrevity", "claude-code/2.1.117", now)
	if !ok || version != "2.1.117" {
		t.Fatalf("first miss = (%q, %v), want (2.1.117, true)", version, ok)
	}

	if _, ok := s.shouldLogRewriteMiss("StripToneBrevity", "claude-code/2.1.117", now.Add(30*time.Minute)); ok {
		t.Fatal("second miss within interval should be suppressed")
	}

	if _, ok := s.shouldLogRewriteMiss("StripOutputEfficiency", "claude-code/2.1.117", now.Add(30*time.Minute)); !ok {
		t.Fatal("different function should log")
	}

	if _, ok := s.shouldLogRewriteMiss("StripToneBrevity", "claude-code/2.2.0", now.Add(30*time.Minute)); !ok {
		t.Fatal("different version should log")
	}

	if _, ok := s.shouldLogRewriteMiss("StripToneBrevity", "claude-code/2.1.117", now.Add(time.Hour)); !ok {
		t.Fatal("same function and version should log again after interval")
	}
}
