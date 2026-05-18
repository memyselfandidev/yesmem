package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildForkRequest_ClonesPrefix(t *testing.T) {
	originalBody := []byte(`{
		"model": "claude-opus-4-6",
		"max_tokens": 8192,
		"stream": true,
		"system": [{"type": "text", "text": "You are helpful."}],
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there"}
		],
		"tools": [{"name": "Bash"}]
	}`)

	cfg := ForkConfig{
		Name:      "test_fork",
		Model:     "claude-sonnet-4-6",
		MaxTokens: 2048,
		Prompt: func(ctx ForkContext) string {
			return "Extract learnings from this conversation."
		},
	}

	ctx := ForkContext{
		OriginalBody:      originalBody,
		AssistantResponse: "Hi there",
		LastExtractedIdx:  0,
	}

	result, err := buildForkRequest(ctx, cfg)
	if err != nil {
		t.Fatalf("buildForkRequest error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Model overridden
	if req["model"] != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %v", req["model"])
	}

	// Stream disabled
	if req["stream"] != false {
		t.Errorf("expected stream=false, got %v", req["stream"])
	}

	// Max tokens overridden
	if int(req["max_tokens"].(float64)) != 2048 {
		t.Errorf("expected max_tokens=2048, got %v", req["max_tokens"])
	}

	// Tools preserved (byte-identical prefix for cache hit with main thread)
	if req["tools"] == nil {
		t.Error("tools must be preserved for cache prefix compatibility")
	}

	// Messages: original + assistant response + fork prompt
	msgs := req["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	// Last message is the fork prompt
	lastMsg := msgs[3].(map[string]any)
	if lastMsg["role"] != "user" {
		t.Errorf("expected last role=user, got %v", lastMsg["role"])
	}

	content := lastMsg["content"].(string)
	if content != "Extract learnings from this conversation." {
		t.Errorf("unexpected fork prompt: %s", content)
	}

	// System prompt preserved (byte-identical for cache hit)
	if req["system"] == nil {
		t.Error("system prompt should be preserved")
	}
}

func TestBuildForkRequest_NormalizesEffort(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "maps xhigh to high",
			input: `{
				"model": "deepseek-v4-pro",
				"max_tokens": 8192,
				"stream": true,
				"output_config": {"effort": "xhigh"},
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
		{
			name: "maps max to high",
			input: `{
				"model": "deepseek-v4-pro",
				"max_tokens": 8192,
				"stream": true,
				"output_config": {"effort": "max"},
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
		{
			name: "does not change empty effort",
			input: `{
				"model": "claude-opus-4-6",
				"max_tokens": 8192,
				"stream": true,
				"output_config": {},
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
		{
			name: "sets high when no output_config",
			input: `{
				"model": "deepseek-v4-pro",
				"max_tokens": 8192,
				"stream": true,
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ForkConfig{
				Name:      "test",
				Model:     "claude-sonnet-4-6",
				MaxTokens: 1024,
				Prompt:    func(ctx ForkContext) string { return "test" },
			}

			result, err := buildForkRequest(ForkContext{OriginalBody: []byte(tt.input)}, cfg)
			if err != nil {
				t.Fatalf("buildForkRequest error: %v", err)
			}

			var req map[string]any
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			oc, ok := req["output_config"].(map[string]any)
			if !ok {
				t.Fatal("expected output_config to exist")
			}

			effort, _ := oc["effort"].(string)
			if effort != "high" {
				t.Errorf("expected effort='high', got %q", effort)
			}
		})
	}
}

func TestBuildForkRequest_StripsAntiDistillation(t *testing.T) {
	originalBody := []byte(`{
		"model": "claude-opus-4-6",
		"max_tokens": 8192,
		"stream": true,
		"anti_distillation": ["fake_tools"],
		"messages": [{"role": "user", "content": "Hello"}]
	}`)

	cfg := ForkConfig{
		Name:      "test",
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Prompt:    func(ctx ForkContext) string { return "test" },
	}

	result, err := buildForkRequest(ForkContext{OriginalBody: originalBody}, cfg)
	if err != nil {
		t.Fatalf("buildForkRequest error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(result, &req)

	if req["anti_distillation"] != nil {
		t.Error("anti_distillation should be stripped from fork request")
	}
}

func TestForkEndToEnd_MockedAPI(t *testing.T) {
	// Test the full flow: buildForkRequest → gate check → prompt with injected IDs → parse response
	originalBody := []byte(`{
		"model": "claude-opus-4-6",
		"max_tokens": 8192,
		"stream": true,
		"system": [{"type": "text", "text": "You are helpful."}],
		"messages": [
			{"role": "user", "content": "How does the proxy work?"},
			{"role": "assistant", "content": "The proxy intercepts requests and manages context."}
		]
	}`)

	cfg := NewExtractAndEvaluateConfig("claude-sonnet-4-6")
	ctx := ForkContext{
		OriginalBody:      originalBody,
		AssistantResponse: "The proxy intercepts requests and manages context.",
		CacheReadTokens:   50000,
		LastExtractedIdx:  0,
		InjectedIDs:       map[int64]string{42: "briefing"},
		Project:           "yesmem",
	}

	// Gate should pass (CacheReadTokens > 0)
	if !cfg.Gate(ctx) {
		t.Error("gate should pass with cache tokens > 0")
	}

	// Build request
	reqBody, err := buildForkRequest(ctx, cfg)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	// Verify request structure
	var req map[string]any
	if err := json.Unmarshal(reqBody, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	msgs := req["messages"].([]any)
	// Original 2 + assistant response + fork prompt = 4
	if len(msgs) != 4 {
		t.Errorf("expected 4 messages, got %d", len(msgs))
	}

	// Verify prompt contains injected IDs
	lastMsg := msgs[3].(map[string]any)
	prompt := lastMsg["content"].(string)
	if !strings.Contains(prompt, "42") {
		t.Error("prompt should reference injected learning ID 42")
	}

	// Simulate a response
	mockResponse := `{
		"learnings": [
			{"content": "Proxy manages context via stubbing and collapsing", "category": "pattern", "entities": ["proxy"]}
		],
		"evaluations": [
			{"learning_id": 42, "verdict": "useful", "reason": "Correctly described proxy architecture", "action": "boost"}
		]
	}`

	result, err := parseExtractionJSON(mockResponse)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(result.Learnings) != 1 {
		t.Errorf("expected 1 learning, got %d", len(result.Learnings))
	}
	if result.Learnings[0].Category != "pattern" {
		t.Errorf("expected category 'pattern', got %q", result.Learnings[0].Category)
	}
	if len(result.Evaluations) != 1 {
		t.Errorf("expected 1 evaluation, got %d", len(result.Evaluations))
	}
	if result.Evaluations[0].Action != "boost" {
		t.Errorf("expected action 'boost', got %q", result.Evaluations[0].Action)
	}
}

func TestParseAnthropicResponse(t *testing.T) {
	body := []byte(`{
		"content": [{"type": "text", "text": "Analysis result"}, {"type": "text", "text": " more"}],
		"usage": {
			"input_tokens": 50000,
			"output_tokens": 2000,
			"cache_read_input_tokens": 49000,
			"cache_creation_input_tokens": 1000
		}
	}`)

	resp, err := parseAnthropicResponse(body)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp.Content != "Analysis result more" {
		t.Errorf("expected 'Analysis result more', got %q", resp.Content)
	}
	if resp.Usage.InputTokens != 50000 {
		t.Errorf("expected 50000 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 49000 {
		t.Errorf("expected 49000 cache read, got %d", resp.Usage.CacheReadInputTokens)
	}
}

func TestParseOpenAIResponse(t *testing.T) {
	body := []byte(`{
		"choices": [
			{"message": {"content": "Extraction result"}}
		],
		"usage": {
			"prompt_tokens": 45000,
			"completion_tokens": 3000,
			"prompt_cache_hit_tokens": 44000,
			"prompt_cache_miss_tokens": 1000
		}
	}`)

	resp, err := parseOpenAIResponse(body)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp.Content != "Extraction result" {
		t.Errorf("expected 'Extraction result', got %q", resp.Content)
	}
	if resp.Usage.InputTokens != 45000 {
		t.Errorf("expected 45000 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 44000 {
		t.Errorf("expected 44000 cache read, got %d", resp.Usage.CacheReadInputTokens)
	}
}

func TestParseOpenAIResponse_Error(t *testing.T) {
	body := []byte(`{
		"error": {"message": "model overloaded"}
	}`)

	_, err := parseOpenAIResponse(body)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestParseAnthropicResponse_Error(t *testing.T) {
	body := []byte(`{
		"error": {"type": "overloaded", "message": "too many requests"}
	}`)

	_, err := parseAnthropicResponse(body)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestNewExtractAndEvaluateConfig_OpenAIFormat(t *testing.T) {
	// APIFormat is no longer set in config — it's auto-detected at runtime.
	// Verify config structure is correct regardless.
	cfg := NewExtractAndEvaluateConfig("deepseek-v4-flash")
	if cfg.Name != "extract_and_evaluate" {
		t.Errorf("expected name 'extract_and_evaluate', got %q", cfg.Name)
	}
	if cfg.Model != "deepseek-v4-flash" {
		t.Errorf("expected model 'deepseek-v4-flash', got %q", cfg.Model)
	}
	if cfg.MaxTokens != 3072 {
		t.Errorf("expected MaxTokens 3072, got %d", cfg.MaxTokens)
	}
	if cfg.Prompt == nil {
		t.Error("expected non-nil Prompt")
	}
	if cfg.ParseResult == nil {
		t.Error("expected non-nil ParseResult")
	}
}

func TestNewExtractAndEvaluateConfig_AnthropicFormat(t *testing.T) {
	// Empty model → inherits from main thread. Format auto-detected.
	cfg := NewExtractAndEvaluateConfig("")
	if cfg.Model != "" {
		t.Errorf("expected empty model, got %q", cfg.Model)
	}
}

func TestBuildForkRequest_PreservesBytePrefix_OpenAI(t *testing.T) {
	// Verify that buildForkRequestOpenAI preserves the byte prefix identical
	// to the main request (messages unchanged before fork prompt), appends
	// the fork prompt as a user message, sets stream=false, and adds
	// response_format for JSON output.
	originalBody := []byte(`{"max_tokens":64000,"messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there"}],"model":"deepseek-v4-pro","stream":true,"tools":[{"type":"function","function":{"name":"Bash","parameters":{}}}]}`)

	cfg := ForkConfig{
		Name:      "extract_test",
		Model:     "",
		MaxTokens: 3072,
		Prompt:    func(ctx ForkContext) string { return "Extract learnings." },
	}

	ctx := ForkContext{
		OriginalBody:      originalBody,
		AssistantResponse: "",
	}

	result, err := buildForkRequest(ctx, cfg)
	if err != nil {
		t.Fatalf("buildForkRequest error: %v", err)
	}

	// Byte prefix before "messages" must be identical
	origMsgIdx := bytes.Index(originalBody, []byte(`"messages"`))
	forkMsgIdx := bytes.Index(result, []byte(`"messages"`))
	if !bytes.Equal(originalBody[:origMsgIdx], result[:forkMsgIdx]) {
		t.Errorf("cache prefix broken: orig=%q, fork=%q", originalBody[:origMsgIdx], result[:forkMsgIdx])
	}

	// Each ORIGINAL message must be byte-identical
	type wrapper struct {
		Messages []json.RawMessage `json:"messages"`
	}
	var origWrap, forkWrap wrapper
	json.Unmarshal(originalBody, &origWrap)
	json.Unmarshal(result, &forkWrap)

	for i := range origWrap.Messages {
		if !bytes.Equal(origWrap.Messages[i], forkWrap.Messages[i]) {
			t.Errorf("message[%d] byte content changed: orig=%s fork=%s", i, origWrap.Messages[i], forkWrap.Messages[i])
		}
	}

	// Fork must have one extra message (the fork prompt)
	if len(forkWrap.Messages) != len(origWrap.Messages)+1 {
		t.Errorf("expected %d messages (orig +1), got %d", len(origWrap.Messages)+1, len(forkWrap.Messages))
	}

	// Last message must be the fork prompt with role=user
	var lastMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	json.Unmarshal(forkWrap.Messages[len(forkWrap.Messages)-1], &lastMsg)
	if lastMsg.Role != "user" {
		t.Errorf("expected last msg role=user, got %q", lastMsg.Role)
	}
	if lastMsg.Content != "Extract learnings." {
		t.Errorf("expected fork prompt, got %q", lastMsg.Content)
	}

	// stream must be false
	var forkMap map[string]any
	json.Unmarshal(result, &forkMap)
	if forkMap["stream"] != false {
		t.Errorf("expected stream=false, got %v", forkMap["stream"])
	}
}

func TestBuildForkRequest_PreservesBytePrefix_OpenAI_NoContextMgmt(t *testing.T) {
	originalBody := []byte(`{"max_tokens":64000,"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there"}],"model":"deepseek-v4-pro","stream":true,"tools":[{"type":"function","function":{"name":"Bash","parameters":{}}}]}`)

	cfg := ForkConfig{
		Name:      "extract_test",
		Model:     "",
		MaxTokens: 3072,
		Prompt:    func(ctx ForkContext) string { return "Extract." },
	}

	ctx := ForkContext{
		OriginalBody:      originalBody,
		AssistantResponse: "",
	}

	result, err := buildForkRequest(ctx, cfg)
	if err != nil {
		t.Fatalf("buildForkRequest error: %v", err)
	}

	origMsgIdx := bytes.Index(originalBody, []byte(`"messages"`))
	forkMsgIdx := bytes.Index(result, []byte(`"messages"`))

	if !bytes.Equal(originalBody[:origMsgIdx], result[:forkMsgIdx]) {
		t.Errorf("cache prefix broken: orig=%q, fork=%q", originalBody[:origMsgIdx], result[:forkMsgIdx])
	}

	// Original messages byte-identical
	type wrapper struct {
		Messages []json.RawMessage `json:"messages"`
	}
	var origWrap, forkWrap wrapper
	json.Unmarshal(originalBody, &origWrap)
	json.Unmarshal(result, &forkWrap)

	for i := range origWrap.Messages {
		if !bytes.Equal(origWrap.Messages[i], forkWrap.Messages[i]) {
			t.Errorf("message[%d] byte content changed", i)
		}
	}
}
