package daemon

import "testing"

func TestFindBashScript_ByName(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "send", Kind: "handler", Runtime: "bash", Body: "echo send"},
			{Name: "recv", Kind: "handler", Runtime: "bash", Body: "echo recv"},
		},
	}
	body, _, ok := findBashScript(meta, "recv")
	if !ok {
		t.Fatal("expected match for recv")
	}
	if body != "echo recv" {
		t.Errorf("body = %q, want %q", body, "echo recv")
	}
}

func TestFindBashScript_FallbackFirstBash(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "tool", Kind: "tool", Runtime: "repl", Body: "x"},
			{Name: "send", Kind: "handler", Runtime: "bash", Body: "echo send"},
			{Name: "recv", Kind: "handler", Runtime: "bash", Body: "echo recv"},
		},
	}
	body, _, ok := findBashScript(meta, "")
	if !ok {
		t.Fatal("expected fallback match")
	}
	if body != "echo send" {
		t.Errorf("body = %q, want %q", body, "echo send")
	}
}

func TestFindBashScript_NoMatchByName(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "send", Kind: "handler", Runtime: "bash", Body: "echo send"},
		},
	}
	_, _, ok := findBashScript(meta, "missing")
	if ok {
		t.Error("expected no match for missing script name")
	}
}

func TestFindBashScript_NoBashScripts(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "tool", Kind: "tool", Runtime: "repl", Body: "x"},
		},
	}
	_, _, ok := findBashScript(meta, "")
	if ok {
		t.Error("expected no fallback when no bash scripts exist")
	}
}

func TestFindBashScript_ByName_PreferBashRuntime(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "send", Kind: "tool", Runtime: "repl", Body: "console.log('x')"},
			{Name: "send", Kind: "handler", Runtime: "bash", Body: "echo send"},
		},
	}
	body, _, ok := findBashScript(meta, "send")
	if !ok {
		t.Fatal("expected match for bash send")
	}
	if body != "echo send" {
		t.Errorf("body = %q, want %q", body, "echo send")
	}
}

func TestFindBashScript_ReturnsSandbox(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "heartbeat", Kind: "handler", Runtime: "bash", Body: "echo ok", Sandbox: "none"},
		},
	}
	body, sandbox, ok := findBashScript(meta, "heartbeat")
	if !ok {
		t.Fatal("expected match")
	}
	if body != "echo ok" {
		t.Errorf("body = %q, want %q", body, "echo ok")
	}
	if sandbox != "none" {
		t.Errorf("sandbox = %q, want %q", sandbox, "none")
	}
}

func TestFindBashScript_EmptySandboxIsInheritSentinel(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "run", Kind: "handler", Runtime: "bash", Body: "echo run", Sandbox: ""},
		},
	}
	body, sandbox, ok := findBashScript(meta, "run")
	if !ok {
		t.Fatal("expected match")
	}
	if body != "echo run" {
		t.Errorf("body = %q, want %q", body, "echo run")
	}
	if sandbox != "" {
		t.Errorf("sandbox = %q, want empty (inherit)", sandbox)
	}
}

func TestFindBashScript_NoMatch(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "send", Kind: "handler", Runtime: "bash", Body: "echo send"},
		},
	}
	_, _, ok := findBashScript(meta, "nonexistent")
	if ok {
		t.Error("expected no match")
	}
}

// TestFindBashScript_FallbackPrefersHandlerOverTool documents the telegram bug:
// when scriptName is empty and the cap has both tool/bash and handler/bash scripts,
// the fallback must pick the handler — scheduled jobs are never tool calls. The
// telegram cap order is [tool/bash send, handler/bash poll, handler/bash reply],
// so a fallback that picks the first bash script silently runs telegram_send.
func TestFindBashScript_FallbackPrefersHandlerOverTool(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "send", Kind: "tool", Runtime: "bash", Body: "echo send"},
			{Name: "poll", Kind: "handler", Runtime: "bash", Body: "echo poll"},
			{Name: "reply", Kind: "handler", Runtime: "bash", Body: "echo reply"},
		},
	}
	body, _, ok := findBashScript(meta, "")
	if !ok {
		t.Fatal("expected fallback match")
	}
	if body != "echo poll" {
		t.Errorf("body = %q, want %q (first handler/bash, not first bash)", body, "echo poll")
	}
}

// TestFindBashScript_FallbackToolBashOnly preserves backward compat: when a cap
// only has tool/bash scripts (no handler/bash), the fallback returns the first
// tool/bash. This matches old single-tool caps that predate the kind split.
func TestFindBashScript_FallbackToolBashOnly(t *testing.T) {
	meta := &CapMeta{
		Scripts: []ScriptMeta{
			{Name: "do_thing", Kind: "tool", Runtime: "bash", Body: "echo legacy"},
		},
	}
	body, _, ok := findBashScript(meta, "")
	if !ok {
		t.Fatal("expected fallback match for tool-only cap")
	}
	if body != "echo legacy" {
		t.Errorf("body = %q, want %q", body, "echo legacy")
	}
}
