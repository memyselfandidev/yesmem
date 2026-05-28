package extraction

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeOpenAIChatURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", defaultOpenAIChatURL},
		{"https://example.test", "https://example.test/v1/chat/completions"},
		{"https://example.test/", "https://example.test/v1/chat/completions"},
		{"https://example.test/v1", "https://example.test/v1/chat/completions"},
		{"https://example.test/v1/chat/completions", "https://example.test/v1/chat/completions"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeOpenAIChatURL(tt.in); got != tt.want {
				t.Errorf("normalizeOpenAIChatURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOpenAIClientCompleteJSON(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`))
	}))
	defer srv.Close()

	oldOnUsage := OnUsage
	defer func() { OnUsage = oldOnUsage }()

	var usageModel string
	var usageIn, usageOut int
	OnUsage = func(model string, inputTokens, outputTokens int) {
		usageModel = model
		usageIn = inputTokens
		usageOut = outputTokens
	}

	client := NewOpenAIClient("sk-openai", "gpt-5-mini", srv.URL, "openai_compatible")
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ok": map[string]any{"type": "boolean"},
		},
		"required":             []string{"ok"},
		"additionalProperties": false,
	}

	out, err := client.CompleteJSON("system prompt", "user prompt", schema, WithMaxTokens(321))
	if err != nil {
		t.Fatalf("CompleteJSON() error = %v", err)
	}
	if out != `{"ok":true}` {
		t.Fatalf("CompleteJSON() = %q", out)
	}
	if gotAuth != "Bearer sk-openai" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotBody["model"] != "gpt-5-mini" {
		t.Fatalf("model = %#v", gotBody["model"])
	}

	msgs, ok := gotBody["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing or not an array: %#v", gotBody["messages"])
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	first, ok := msgs[0].(map[string]any)
	if !ok || first["role"] != "system" || first["content"] != "system prompt" {
		t.Fatalf("first message = %#v", first)
	}
	second, ok := msgs[1].(map[string]any)
	if !ok || second["role"] != "user" || second["content"] != "user prompt" {
		t.Fatalf("second message = %#v", second)
	}

	if gotBody["max_tokens"] != float64(321) {
		t.Fatalf("max_tokens = %#v, want 321", gotBody["max_tokens"])
	}
	if gotBody["store"] != false {
		t.Fatalf("store = %#v, want false", gotBody["store"])
	}

	respFmt, ok := gotBody["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format missing: %#v", gotBody["response_format"])
	}
	if respFmt["type"] != "json_schema" {
		t.Fatalf("response_format.type = %#v", respFmt["type"])
	}

	if usageModel != "gpt-5-mini" || usageIn != 12 || usageOut != 7 {
		t.Fatalf("usage callback = (%q, %d, %d)", usageModel, usageIn, usageOut)
	}
}

func TestOpenAIClientComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello world"}}]}`))
	}))
	defer srv.Close()

	client := NewOpenAIClient("sk-openai", "deepseek-chat", srv.URL, "openai")
	out, err := client.Complete("system", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if out != "hello world" {
		t.Fatalf("Complete() = %q, want %q", out, "hello world")
	}
}

func TestPricingForGPTModels(t *testing.T) {
	tests := []struct {
		model      string
		inputPerM  float64
		outputPerM float64
	}{
		{"gpt-5-mini", 0.25, 2.0},
		{"gpt-5.2", 1.75, 14.0},
		{"gpt-5.2-codex", 1.75, 14.0},
		{"gpt-5.4", 2.5, 15.0},
	}

	for _, tt := range tests {
		in, out := PricingForModel(tt.model)
		if in != tt.inputPerM || out != tt.outputPerM {
			t.Errorf("PricingForModel(%q) = (%v, %v), want (%v, %v)", tt.model, in, out, tt.inputPerM, tt.outputPerM)
		}
	}
}
