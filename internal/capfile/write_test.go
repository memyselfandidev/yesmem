package capfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender_RoundtripSingleScriptCap(t *testing.T) {
	original := &CapFile{
		Name:        "hello",
		Description: "Test cap",
		Version:     1,
		Tags:        []string{"demo"},
		Purpose:     "A minimal hello cap.",
		Scripts: []Script{{
			Name:    "hello",
			Kind:    "tool",
			Runtime: "repl",
			Lang:    "javascript",
			Body:    "async ({ name }) => ({ greeting: 'Hello ' + name })",
			Schema:  `{"type":"object","properties":{"name":{}}}`,
		}},
	}

	rendered := Render(original)
	parsed, err := Parse(rendered)
	if err != nil {
		t.Fatalf("Parse rendered output: %v\nrendered:\n%s", err, rendered)
	}

	if parsed.Name != original.Name {
		t.Errorf("Name lost: %q -> %q", original.Name, parsed.Name)
	}
	if len(parsed.Scripts) != 1 {
		t.Fatalf("Scripts count: got %d, want 1", len(parsed.Scripts))
	}
	got := parsed.Scripts[0]
	if got.Name != "hello" || got.Kind != "tool" || got.Runtime != "repl" {
		t.Errorf("Script fields lost: %+v", got)
	}
}

func TestRender_BundleCap(t *testing.T) {
	cf := &CapFile{
		Name:        "telegram",
		Description: "Telegram bundle",
		Version:     1,
		Purpose:     "Telegram bundle.",
		Scripts: []Script{
			{
				Name:    "telegram_send",
				Kind:    "tool",
				Runtime: "bash",
				Lang:    "bash",
				Body:    "echo $TEXT",
				Schema:  `{"type":"object","properties":{"text":{"type":"string"}}}`,
			},
			{
				Name:    "telegram_poll",
				Kind:    "handler",
				Runtime: "bash",
				Lang:    "bash",
				Body:    "echo poll",
			},
		},
	}
	out := string(Render(cf))

	for _, want := range []string{
		"## Scripts",
		"### telegram_send",
		"### telegram_poll",
		"kind: tool",
		"kind: handler",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n---\n%s\n---", want, out)
		}
	}

	// Round-trip
	parsed, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("round-trip Parse: %v\noutput:\n%s", err, out)
	}
	if len(parsed.Scripts) != 2 {
		t.Errorf("round-trip Scripts: got %d, want 2", len(parsed.Scripts))
	}
}

func TestRender_OmitsDatabaseWhenEmpty(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
	}
	out := string(Render(cf))
	if strings.Contains(out, "## Database") {
		t.Errorf("Database section emitted for stateless cap:\n%s", out)
	}
}

func TestRender_EmitsDatabaseWhenSQLPresent(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
		DatabaseSQL: "CREATE TABLE IF NOT EXISTS cap_x__t (id TEXT PRIMARY KEY);",
	}
	out := string(Render(cf))
	if !strings.Contains(out, "## Database") {
		t.Errorf("Database section missing despite SQL present:\n%s", out)
	}
	if !strings.Contains(out, "```sql") {
		t.Errorf("SQL fence missing:\n%s", out)
	}
}

func TestRender_OmitsActionsWhenEmpty(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
	}
	out := string(Render(cf))
	if strings.Contains(out, "## Actions") {
		t.Errorf("Actions section emitted when empty:\n%s", out)
	}
}

func TestRender_EmitsActionsSorted(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
		Actions: map[string]string{
			"teardown": "Drop config row.",
			"setup":    "Set token.",
		},
	}
	out := string(Render(cf))
	setupIdx := strings.Index(out, "### setup")
	teardownIdx := strings.Index(out, "### teardown")
	if setupIdx < 0 || teardownIdx < 0 {
		t.Fatalf("setup/teardown subsections missing:\n%s", out)
	}
	if setupIdx > teardownIdx {
		t.Errorf("setup must appear before teardown (sorted), got setup=%d teardown=%d", setupIdx, teardownIdx)
	}
}

func TestRender_EmitsRequiresWhenSet(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Requires:    []string{"store", "web"},
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
	}
	out := string(Render(cf))
	if !strings.Contains(out, "requires: [store, web]") {
		t.Errorf("requires: line missing:\n%s", out)
	}
}

func TestRender_DoesNotEmitCapLevelRuntimeOrSchema(t *testing.T) {
	cf := &CapFile{
		Name:        "x",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "x", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body:   "async () => ({})",
			Schema: `{"type":"object"}`,
		}},
	}
	out := string(Render(cf))
	for _, banned := range []string{"\nruntime:", "\nschema:"} {
		// Frontmatter should not contain these — they're per-script now.
		fmEnd := strings.Index(out, "\n---\n\n##")
		if fmEnd < 0 {
			fmEnd = len(out)
		}
		fm := out[:fmEnd]
		if strings.Contains(fm, banned) {
			t.Errorf("frontmatter contains banned %q:\n%s", banned, fm)
		}
	}
}

func TestWriteFile_AtomicCreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mycap")
	cf := &CapFile{
		Name:        "mycap",
		Description: "x",
		Purpose:     "x",
		Scripts: []Script{{
			Name: "mycap", Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async () => ({})",
		}},
	}
	if err := WriteFile(cf, target); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "CAP.md"))
	if err != nil {
		t.Fatalf("read CAP.md: %v", err)
	}
	if !strings.Contains(string(data), "## Scripts") {
		t.Errorf("written file missing Scripts section:\n%s", data)
	}
}
