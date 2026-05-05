package daemon

import (
	"strings"
	"testing"
)

func TestParseCapMeta_Valid(t *testing.T) {
	json := `{
		"cap_name":"reddit_fetch",
		"cap_description":"Fetch Reddit posts",
		"cap_scripts":[
			{"name":"reddit_fetch","kind":"tool","runtime":"bash","body":"curl -s \"$URL.json\"","schema":"{\"type\":\"object\"}"}
		],
		"cap_tags":["web","reddit"],
		"cap_version":1,
		"cap_tested":true
	}`
	meta, err := ParseCapMeta(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Name != "reddit_fetch" {
		t.Errorf("expected name 'reddit_fetch', got %q", meta.Name)
	}
	if len(meta.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(meta.Scripts))
	}
	if meta.Scripts[0].Body == "" {
		t.Error("expected non-empty Body")
	}
	if meta.Scripts[0].Runtime != "bash" {
		t.Errorf("expected runtime bash, got %q", meta.Scripts[0].Runtime)
	}
	if len(meta.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(meta.Tags))
	}
	if meta.Version != 1 {
		t.Error("expected version 1")
	}
	if !meta.Tested {
		t.Error("expected tested=true")
	}
}

func TestParseCapMeta_MissingName(t *testing.T) {
	json := `{"cap_description":"No name","cap_scripts":[{"name":"x","kind":"tool","runtime":"bash","body":"echo hi","schema":"{}"}]}`
	_, err := ParseCapMeta(json)
	if err == nil {
		t.Fatal("expected error for missing cap_name")
	}
}

func TestParseCapMeta_MissingScripts(t *testing.T) {
	json := `{"cap_name":"test","cap_description":"Test"}`
	_, err := ParseCapMeta(json)
	if err == nil {
		t.Fatal("expected error when no scripts are provided")
	}
}

func TestParseCapMeta_EmptyString(t *testing.T) {
	_, err := ParseCapMeta("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestCapMeta_ToJSON(t *testing.T) {
	meta := CapMeta{
		Name:        "test_tool",
		Description: "A test tool",
		Scripts: []ScriptMeta{
			{Name: "test_tool", Kind: "tool", Runtime: "bash", Body: "echo hello", Schema: "{}"},
		},
		Tags:    []string{"test"},
		Version: 1,
		Tested:  true,
	}
	s, err := meta.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := ParseCapMeta(s)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if parsed.Name != meta.Name {
		t.Errorf("name mismatch after round-trip: %q vs %q", parsed.Name, meta.Name)
	}
	if len(parsed.Scripts) != 1 {
		t.Fatalf("expected 1 script after round-trip, got %d", len(parsed.Scripts))
	}
	if parsed.Scripts[0].Body != meta.Scripts[0].Body {
		t.Errorf("script body mismatch after round-trip")
	}
}

func TestCapMeta_HasTag(t *testing.T) {
	meta := CapMeta{
		Name: "x",
		Scripts: []ScriptMeta{
			{Name: "x", Kind: "tool", Runtime: "bash", Body: "y", Schema: "{}"},
		},
		Tags: []string{"web", "reddit", "fetch"},
	}
	if !meta.HasTag("reddit") {
		t.Error("expected HasTag('reddit') to be true")
	}
	if meta.HasTag("nonexistent") {
		t.Error("expected HasTag('nonexistent') to be false")
	}
}

func TestParseCapMeta_ScriptSandboxRoundTrip(t *testing.T) {
	meta := CapMeta{
		Name: "sandbox_cap",
		Scripts: []ScriptMeta{
			{Name: "sandbox_cap", Kind: "tool", Runtime: "bash", Body: "echo hi", Schema: "{}", Sandbox: "none"},
		},
	}
	s, err := meta.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	parsed, err := ParseCapMeta(s)
	if err != nil {
		t.Fatalf("ParseCapMeta failed: %v", err)
	}
	if parsed.Scripts[0].Sandbox != "none" {
		t.Errorf("expected sandbox 'none', got %q", parsed.Scripts[0].Sandbox)
	}
}

func TestParseCapMeta_ScriptSandboxAllValidValues(t *testing.T) {
	cases := []string{"", "none", "standard", "strict"}
	for _, val := range cases {
		t.Run("sandbox="+val, func(t *testing.T) {
			meta := CapMeta{
				Name: "cap",
				Scripts: []ScriptMeta{
					{Name: "cap", Kind: "tool", Runtime: "bash", Body: "echo hi", Schema: "{}", Sandbox: val},
				},
			}
			s, err := meta.ToJSON()
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}
			_, err = ParseCapMeta(s)
			if err != nil {
				t.Errorf("expected valid sandbox %q to parse cleanly, got: %v", val, err)
			}
		})
	}
}

func TestParseCapMeta_ScriptSandboxInvalid(t *testing.T) {
	raw := `{"cap_name":"cap","cap_scripts":[{"name":"cap","kind":"tool","runtime":"bash","body":"echo hi","schema":"{}","sandbox":"yolo"}]}`
	_, err := ParseCapMeta(raw)
	if err == nil {
		t.Fatal("expected error for invalid sandbox value")
	}
	if !strings.Contains(err.Error(), "sandbox") {
		t.Errorf("expected error to mention 'sandbox', got: %v", err)
	}
}

func TestCapMeta_ToolScripts(t *testing.T) {
	meta := CapMeta{
		Name: "bundle",
		Scripts: []ScriptMeta{
			{Name: "tool_a", Kind: "tool", Runtime: "repl", Body: "async () => 1", Schema: "{}"},
			{Name: "helper", Kind: "handler", Runtime: "repl", Body: "async () => 2"},
			{Name: "tool_b", Kind: "tool", Runtime: "bash", Body: "echo hi", Schema: "{}"},
		},
	}
	tools := meta.ToolScripts()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tool scripts, got %d", len(tools))
	}
	if tools[0].Name != "tool_a" || tools[1].Name != "tool_b" {
		t.Errorf("unexpected tool ordering: %v", tools)
	}
}
