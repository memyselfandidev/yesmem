package mcp

import (
	"testing"
)

func TestRememberToolExposesOriginParameter(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	tool := s.srv.GetTool("remember")
	if tool == nil {
		t.Fatal("GetTool(remember) returned nil — tool not registered")
	}

	props := tool.Tool.InputSchema.Properties
	if _, ok := props["origin"]; !ok {
		t.Fatalf("remember tool schema missing 'origin' property; have keys: %v", keysOf(props))
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
