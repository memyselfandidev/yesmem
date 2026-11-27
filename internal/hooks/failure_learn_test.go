package hooks

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/carsteneu/yesmem/internal/storage"
)

func openHookStore(t *testing.T, dir string) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(dir, "yesmem.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func readFailure(output string) *FailureInput {
	return &FailureInput{
		SessionID:  "sess-failure-test",
		ToolName:   "Read",
		ToolInput:  json.RawMessage(`{"file_path":"/repo/main.go"}`),
		ToolOutput: output,
	}
}

// TestLearnFromAnyFailureKeepsDistinctFailures is the same defect as in
// learn.go, at the third call site: the 0.5 containment rule dropped a failure
// whose command/file part is identical and whose error part is the new
// information, and bumped the counters of the older row instead.
func TestLearnFromAnyFailureKeepsDistinctFailures(t *testing.T) {
	dir := t.TempDir()

	learnFromAnyFailure(dir, readFailure("open /repo/main.go: permission denied"), "/repo/main.go")
	learnFromAnyFailure(dir, readFailure("open /repo/main.go: no such file or directory"), "/repo/main.go")

	rows, err := openHookStore(t, dir).GetActiveLearnings("gotcha", "", "", "", 0)
	if err != nil {
		t.Fatalf("get active learnings: %v", err)
	}
	if len(rows) != 2 {
		var contents []string
		for _, r := range rows {
			contents = append(contents, r.Content)
		}
		t.Fatalf("gotcha rows: got %d %v, want 2 — the error text is the information", len(rows), contents)
	}
}

// TestLearnFromAnyFailureDeduplicatesIdenticalFailure keeps the case the old
// rule got right: an identical failure is one observation and feeds both
// counters of the existing row.
func TestLearnFromAnyFailureDeduplicatesIdenticalFailure(t *testing.T) {
	dir := t.TempDir()

	learnFromAnyFailure(dir, readFailure("open /repo/main.go: permission denied"), "/repo/main.go")
	learnFromAnyFailure(dir, readFailure("open /repo/main.go: permission denied"), "/repo/main.go")

	rows, err := openHookStore(t, dir).GetActiveLearnings("gotcha", "", "", "", 0)
	if err != nil {
		t.Fatalf("get active learnings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("gotcha rows: got %d, want 1", len(rows))
	}
	if rows[0].MatchCount != 1 {
		t.Errorf("match_count: got %d, want 1", rows[0].MatchCount)
	}
	if rows[0].FailCount != 1 {
		t.Errorf("fail_count: got %d, want 1", rows[0].FailCount)
	}
}
