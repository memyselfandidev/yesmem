package proxy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// First user message must be content-blanked to "-" after collapse.
// Rationale: messages[0] survives every Stubify/Compress pass (Zone-1 protect).
// Anthropic's API has system as a separate top-level field, so messages[0]
// is in fact the original first user turn. Without blanking, the model latches
// onto stale opening framing in every collapsed session. Blanking content while
// preserving role keeps the cache prefix byte-stable on subsequent requests
// (one-time cache invalidation on first collapse after deploy, then steady).
func TestCollapseOldMessages_BlanksFirstMessageContent(t *testing.T) {
	msgs := make([]any, 30)
	msgs[0] = map[string]any{
		"role":    "user",
		"content": "Original task framing — investigate session cc0ba29d and report back",
	}
	for i := 1; i < 30; i++ {
		msgs[i] = map[string]any{
			"role":    "user",
			"content": "later message",
		}
	}

	result := CollapseOldMessages(msgs, msgs, 10, time.Time{}, time.Time{}, nil, nil)

	first, ok := result[0].(map[string]any)
	if !ok {
		t.Fatalf("result[0] not a map: %T", result[0])
	}
	if first["role"] != "user" {
		t.Errorf("role mutated: got %v want user", first["role"])
	}
	if got := first["content"]; got != "-" {
		t.Errorf("content not blanked: got %q want \"-\"", got)
	}
}

// Original m[0] content must NOT leak anywhere in the collapsed output.
// This is the actual user-facing problem: Claude latches onto stale framing
// even after collapse because the first message is preserved verbatim.
func TestCollapseOldMessages_FirstMessageContentDoesNotLeak(t *testing.T) {
	const originalFraming = "DELETE-ALL-DATA-AT-MIDNIGHT-uniquemarker42"
	msgs := make([]any, 30)
	msgs[0] = map[string]any{
		"role":    "user",
		"content": originalFraming,
	}
	for i := 1; i < 30; i++ {
		msgs[i] = map[string]any{
			"role":    "user",
			"content": "later",
		}
	}

	result := CollapseOldMessages(msgs, msgs, 10, time.Time{}, time.Time{}, nil, nil)
	full, _ := json.Marshal(result)
	if strings.Contains(string(full), originalFraming) {
		t.Errorf("original m[0] framing leaked into collapsed result: %s", full)
	}
}

// Cache prefix stability: same input must produce byte-identical result[0]
// across separate calls. The freeze pipeline relies on this for prefix-hash
// validation and cache_control breakpoint hits.
func TestCollapseOldMessages_BlankedFirstMessageIsByteStable(t *testing.T) {
	build := func() []any {
		out := make([]any, 30)
		out[0] = map[string]any{"role": "user", "content": "first"}
		for i := 1; i < 30; i++ {
			out[i] = map[string]any{"role": "user", "content": "msg"}
		}
		return out
	}

	a := CollapseOldMessages(build(), build(), 10, time.Time{}, time.Time{}, nil, nil)
	b := CollapseOldMessages(build(), build(), 10, time.Time{}, time.Time{}, nil, nil)

	aJSON, _ := json.Marshal(a[0])
	bJSON, _ := json.Marshal(b[0])
	if string(aJSON) != string(bJSON) {
		t.Errorf("m[0] not byte-stable across calls:\n  a=%s\n  b=%s", aJSON, bJSON)
	}
}

// Idempotence: calling CollapseOldMessages on already-blanked m[0] must be a
// no-op for the first slot (still "-", role preserved).
func TestCollapseOldMessages_BlankIsIdempotent(t *testing.T) {
	msgs := make([]any, 30)
	msgs[0] = map[string]any{"role": "user", "content": "-"}
	for i := 1; i < 30; i++ {
		msgs[i] = map[string]any{"role": "user", "content": "msg"}
	}

	result := CollapseOldMessages(msgs, msgs, 10, time.Time{}, time.Time{}, nil, nil)
	first, _ := result[0].(map[string]any)
	if first["content"] != "-" {
		t.Errorf("idempotence broken: got %v", first["content"])
	}
	if first["role"] != "user" {
		t.Errorf("role mutated on idempotent call: %v", first["role"])
	}
}

// On the early-return path (cutoffIdx out of range) the function returns its
// input unchanged. m[0] must stay verbatim then — no collapse happened, so
// nothing is blanked.
func TestCollapseOldMessages_NoBlankOnEarlyReturn(t *testing.T) {
	msgs := make([]any, 5)
	msgs[0] = map[string]any{"role": "user", "content": "real opening, keep it"}
	for i := 1; i < 5; i++ {
		msgs[i] = map[string]any{"role": "user", "content": "msg"}
	}

	// cutoffIdx == 0 is below minCollapseMessages → early return
	result := CollapseOldMessages(msgs, msgs, 0, time.Time{}, time.Time{}, nil, nil)
	first, _ := result[0].(map[string]any)
	if first["content"] != "real opening, keep it" {
		t.Errorf("blank applied on early-return (cutoff=0): got %v", first["content"])
	}

	// cutoffIdx >= len(modified) → early return
	result2 := CollapseOldMessages(msgs, msgs, len(msgs), time.Time{}, time.Time{}, nil, nil)
	first2, _ := result2[0].(map[string]any)
	if first2["content"] != "real opening, keep it" {
		t.Errorf("blank applied on early-return (cutoff=len): got %v", first2["content"])
	}
}
