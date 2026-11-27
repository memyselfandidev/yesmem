package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/carsteneu/yesmem/internal/models"
	"github.com/carsteneu/yesmem/internal/storage"
	"github.com/carsteneu/yesmem/internal/textutil"
)

// seedLearning stores a learning directly through the store, bypassing
// handleRemember. It stands in for an entry that already exists in the memory
// when a new remember() call arrives.
func seedLearning(t *testing.T, s *storage.Store, category, content string) int64 {
	t.Helper()
	id, err := s.InsertLearning(&models.Learning{
		Category:   category,
		Content:    content,
		Confidence: 1.0,
		CreatedAt:  time.Now(),
		ModelUsed:  "seed",
		Source:     "user_stated",
	})
	if err != nil {
		t.Fatalf("seed learning: %v", err)
	}
	return id
}

func rememberCall(t *testing.T, h *Handler, params map[string]any) map[string]any {
	t.Helper()
	resp := h.Handle(Request{Method: "remember", Params: params})
	if resp.Error != "" {
		t.Fatalf("remember error: %s", resp.Error)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal remember response: %v (raw %s)", err, string(resp.Result))
	}
	return result
}

func activeLearningCount(t *testing.T, s *storage.Store, project string) int {
	t.Helper()
	rows, err := s.GetActiveLearnings("", project, "", "", 0)
	if err != nil {
		t.Fatalf("get active learnings: %v", err)
	}
	return len(rows)
}

func matchCount(t *testing.T, s *storage.Store, id int64) int {
	t.Helper()
	l, err := s.GetLearning(id)
	if err != nil {
		t.Fatalf("get learning #%d: %v", id, err)
	}
	return l.MatchCount
}

// TestHandleRememberNearDuplicateIsRejectedExplicitly covers mode B (a foreign
// entry that merely shares vocabulary) and mode C (the caller's own earlier
// entry on the same topic). Neither may be swallowed: the response has to say
// that nothing was stored, and match_count of the existing entry must stay put
// — bumping it strengthens an entry the new text was not about.
func TestHandleRememberNearDuplicateIsRejectedExplicitly(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		newText  string
	}{
		{
			name:     "mode B foreign entry sharing vocabulary",
			existing: "Verification doctrine (GLOBAL): a claim is only verified when a command proves it; a check without a command is not verification",
			newText:  "check: claim verification needs a command, not a doctrine",
		},
		{
			name:     "mode C own earlier entry on the same topic",
			existing: "the daemon restarts on deploy and the schedule keeps running",
			newText:  "the daemon restarts on deploy and the schedule keeps running until the next tick",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s := mustHandler(t)
			seedID := seedLearning(t, s, "decision", tc.existing)

			sim := textutil.TokenSimilarity(textutil.Tokenize(tc.newText), textutil.Tokenize(tc.existing))
			if sim < 0.5 {
				t.Fatalf("scenario guard: similarity %.2f < 0.5, this case no longer exercises the dedup threshold", sim)
			}

			result := rememberCall(t, h, map[string]any{
				"text":     tc.newText,
				"category": "decision",
				"project":  "yesmem",
			})

			if stored, ok := result["stored"].(bool); !ok || stored {
				t.Errorf("stored: got %v (present=%v), want false — the new text was not written", result["stored"], ok)
			}
			if got, want := result["duplicate_of"], float64(seedID); got != want {
				t.Errorf("duplicate_of: got %v, want %v", got, want)
			}
			msg, _ := result["message"].(string)
			if !strings.Contains(msg, "NOT stored") {
				t.Errorf("message must say the new text was not stored: got %q", msg)
			}
			if strings.Contains(msg, "Bumped") {
				t.Errorf("message must not claim a match_count bump: got %q", msg)
			}
			if !strings.Contains(msg, "force=true") {
				t.Errorf("message must name a way to store the text anyway: got %q", msg)
			}
			if got := matchCount(t, s, seedID); got != 0 {
				t.Errorf("match_count of #%d: got %d, want 0 — a discarded text must not strengthen an entry", seedID, got)
			}
			if got := activeLearningCount(t, s, "yesmem"); got != 1 {
				t.Errorf("active learnings: got %d, want 1 (no silent insert, no duplicate row)", got)
			}
		})
	}
}

// TestHandleRememberCorrectionIsStored covers mode A: the new text contradicts
// the existing entry. Opposing polarity means it is a correction, not a
// duplicate — even at similarity 1.0. The contradicted entry must stay
// untouched (no supersede, no noise bump): the fix must not rewrite memory.
func TestHandleRememberCorrectionIsStored(t *testing.T) {
	h, s := mustHandler(t)

	oldID := seedLearning(t, s, "gotcha", "diff_scope ist nach dem Merge blind")
	newText := "diff_scope ist nach dem Merge nicht blind"

	sim := textutil.TokenSimilarity(textutil.Tokenize(newText), textutil.Tokenize("diff_scope ist nach dem Merge blind"))
	if sim < 0.5 {
		t.Fatalf("scenario guard: similarity %.2f < 0.5", sim)
	}

	result := rememberCall(t, h, map[string]any{
		"text":     newText,
		"category": "gotcha",
		"project":  "yesmem",
	})

	id, _ := result["id"].(float64)
	if id <= 0 {
		t.Fatalf("correction was not stored: id=%v message=%v", result["id"], result["message"])
	}
	if got := int64(id); got == oldID {
		t.Fatalf("correction returned the old id #%d — it was treated as a duplicate", oldID)
	}

	stored, err := s.GetLearning(int64(id))
	if err != nil {
		t.Fatalf("get stored correction: %v", err)
	}
	if stored.Content != newText {
		t.Errorf("stored content: got %q, want %q", stored.Content, newText)
	}
	msg, _ := result["message"].(string)
	if !strings.Contains(msg, fmt.Sprintf("#%d", oldID)) {
		t.Errorf("message must name the contradicted entry #%d: got %q", oldID, msg)
	}

	old, err := s.GetLearning(oldID)
	if err != nil {
		t.Fatalf("get old learning: %v", err)
	}
	if old.SupersededBy != nil {
		t.Errorf("old entry was superseded (by #%d) — the fix must not rewrite existing memory", *old.SupersededBy)
	}
	if old.NoiseCount != 0 {
		t.Errorf("old entry noise_count: got %d, want 0", old.NoiseCount)
	}
	if old.MatchCount != 0 {
		t.Errorf("old entry match_count: got %d, want 0", old.MatchCount)
	}
}

// TestHandleRememberForceStoresNearDuplicate proves the documented escape hatch:
// when the caller knows the text carries new information, force=true stores it.
func TestHandleRememberForceStoresNearDuplicate(t *testing.T) {
	h, s := mustHandler(t)

	existing := "the daemon restarts on deploy and the schedule keeps running"
	seedID := seedLearning(t, s, "decision", existing)
	newText := "the daemon restarts on deploy and the schedule keeps running until the next tick"

	sim := textutil.TokenSimilarity(textutil.Tokenize(newText), textutil.Tokenize(existing))
	if sim < 0.5 {
		t.Fatalf("scenario guard: similarity %.2f < 0.5", sim)
	}

	result := rememberCall(t, h, map[string]any{
		"text":     newText,
		"category": "decision",
		"project":  "yesmem",
		"force":    true,
	})

	id, _ := result["id"].(float64)
	if id <= 0 {
		t.Fatalf("force=true did not store the text: id=%v message=%v", result["id"], result["message"])
	}
	if int64(id) == seedID {
		t.Fatalf("force=true returned the existing id #%d", seedID)
	}
	if got := matchCount(t, s, seedID); got != 0 {
		t.Errorf("match_count of #%d: got %d, want 0", seedID, got)
	}
	if got := activeLearningCount(t, s, "yesmem"); got != 2 {
		t.Errorf("active learnings: got %d, want 2 (seeded + forced)", got)
	}
}

// TestHandleRememberExactDuplicateReportsNothingNew pins the one case where a
// match_count bump is legitimate: the byte-identical text. Nothing is lost —
// the text is already in the memory verbatim — and the response says so
// instead of pretending a new learning was saved.
func TestHandleRememberExactDuplicateReportsNothingNew(t *testing.T) {
	h, s := mustHandler(t)

	text := "Exact duplicate keeps the memory verbatim"
	first := rememberCall(t, h, map[string]any{"text": text, "category": "pattern", "project": "yesmem"})
	firstID := int64(first["id"].(float64))

	second := rememberCall(t, h, map[string]any{"text": text, "category": "pattern", "project": "yesmem"})
	if got, want := second["id"], float64(firstID); got != want {
		t.Errorf("id: got %v, want %v (the existing entry)", got, want)
	}
	if stored, ok := second["stored"].(bool); !ok || stored {
		t.Errorf("stored: got %v (present=%v), want false", second["stored"], ok)
	}
	msg, _ := second["message"].(string)
	if !strings.Contains(strings.ToLower(msg), "identical") && !strings.Contains(msg, "NOT stored") {
		t.Errorf("message must state that no new learning was written: got %q", msg)
	}
	if got := matchCount(t, s, firstID); got != 1 {
		t.Errorf("match_count of #%d: got %d, want 1 (identical text is a genuine reinforcement)", firstID, got)
	}
	if got := activeLearningCount(t, s, "yesmem"); got != 1 {
		t.Errorf("active learnings: got %d, want 1", got)
	}
}
