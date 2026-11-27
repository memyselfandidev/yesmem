package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/carsteneu/yesmem/internal/storage"
)

// runLearnHook feeds one hook payload through RunLearn and returns the store
// afterwards.
func runLearnHook(t *testing.T, dir string, payload map[string]any) *storage.Store {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal hook payload: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.Write(encoded); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	RunLearn(dir)
	r.Close()

	store, err := storage.Open(filepath.Join(dir, "yesmem.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func bashFailure(command, output string) map[string]any {
	return map[string]any{
		"session_id":      "sess-hook-test",
		"hook_event_name": "PostToolUseFailure",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
		"tool_output":     output,
	}
}

// TestRunLearnKeepsDistinctFailuresOfTheSameCommand is the defect the old
// >= 0.5 fuzzy dedup created: two failures of the same command differ exactly
// where the information lives (the error), yet one was thrown away and the
// other got its match_count bumped.
func TestRunLearnKeepsDistinctFailuresOfTheSameCommand(t *testing.T) {
	dir := t.TempDir()

	runLearnHook(t, dir, bashFailure("go build ./...", "undefined: foo"))
	store := runLearnHook(t, dir, bashFailure("go build ./...", "undefined: bar"))

	rows, err := store.GetActiveLearnings("gotcha", "", "", "", 0)
	if err != nil {
		t.Fatalf("get active learnings: %v", err)
	}
	if len(rows) != 2 {
		var contents []string
		for _, r := range rows {
			contents = append(contents, r.Content)
		}
		t.Fatalf("gotcha rows: got %d %v, want 2 — both failures carry distinct information", len(rows), contents)
	}
	for _, r := range rows {
		if r.MatchCount != 0 {
			t.Errorf("match_count of %q: got %d, want 0", r.Content, r.MatchCount)
		}
	}
}

// TestRunLearnDeduplicatesIdenticalFailure is the case where the old behaviour
// stays right: an identical error is the same observation, so it reinforces the
// existing row instead of adding a second one.
func TestRunLearnDeduplicatesIdenticalFailure(t *testing.T) {
	dir := t.TempDir()

	runLearnHook(t, dir, bashFailure("go build ./...", "undefined: foo"))
	store := runLearnHook(t, dir, bashFailure("go build ./...", "undefined: foo"))

	rows, err := store.GetActiveLearnings("gotcha", "", "", "", 0)
	if err != nil {
		t.Fatalf("get active learnings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("gotcha rows: got %d, want 1 (identical failure is one observation)", len(rows))
	}
	if rows[0].MatchCount != 1 {
		t.Errorf("match_count: got %d, want 1", rows[0].MatchCount)
	}
}
