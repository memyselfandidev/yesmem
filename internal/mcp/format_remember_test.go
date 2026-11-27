package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFormatRememberRejectionIsNotRenderedAsSaved guards the reporting seam: a
// remember() call that stored nothing must not reach the caller as
// "✓ Learning #N saved". The model reads this string, so a wrong rendering is
// the same defect as a wrong store — the text disappears and the caller
// believes it is in the memory. The payload carries the id of the entry the
// text was mistaken for, exactly as the dedup path returns it today.
func TestFormatRememberRejectionIsNotRenderedAsSaved(t *testing.T) {
	raw := json.RawMessage(`{"id":4012,"stored":false,"deduplicated":true,"duplicate_of":4012,"similarity":0.62,"message":"NOT stored. The new text is a near-duplicate of #4012 (similarity 0.62). Nothing was written; store it with force=true if it carries new information."}`)

	got := formatRemember(raw)

	if strings.Contains(got, "✓") || strings.Contains(got, "saved") {
		t.Errorf("rejection rendered as a save: %q", got)
	}
	if !strings.Contains(got, "NOT stored") || !strings.Contains(got, "#4012") {
		t.Errorf("rejection rendering lost the reason: %q", got)
	}
}

// TestFormatRememberCorrectionNoteReachesTheCaller pins the other half of the
// reporting seam: when a correction was stored, the note carries the entry it
// contradicts — and the model reads only this rendering, not the JSON fields.
func TestFormatRememberCorrectionNoteReachesTheCaller(t *testing.T) {
	raw := json.RawMessage(`{"id":4310,"stored":true,"category":"gotcha","project":"yesmem","content":"diff_scope ist nach dem Merge nicht blind","model_used":"self","contradicts":4014,"similarity":0.92,"message":"Learning #4310 saved (gotcha, model=self). Stored although it is 0.92 similar to #4014: the two texts have opposite polarity, so this is a correction, not a duplicate. #4014 was left untouched."}`)

	got := formatRemember(raw)

	if !strings.Contains(got, "Contradicts: #4014") {
		t.Errorf("correction target missing from the rendering: %q", got)
	}
	if !strings.Contains(got, "correction, not a duplicate") {
		t.Errorf("the note explaining the stored correction is missing: %q", got)
	}
}

func TestFormatRememberSavedStillRendersSaved(t *testing.T) {
	raw := json.RawMessage(`{"id":7,"category":"gotcha","project":"yesmem","content":"the diff shows what changed","model_used":"self","message":"Learning #7 saved (gotcha, model=self)."}`)

	got := formatRemember(raw)

	if !strings.Contains(got, "Learning #7 saved") {
		t.Errorf("saved learning not rendered as saved: %q", got)
	}
}

// TestFormatRememberLegacyPayloadWithoutStoredField keeps the format compatible
// with responses that predate the stored flag.
func TestFormatRememberLegacyPayloadWithoutStoredField(t *testing.T) {
	raw := json.RawMessage(`{"id":9,"category":"decision","content":"deploy restarts the daemon","message":"Learning #9 saved (decision, model=self)."}`)

	got := formatRemember(raw)

	if !strings.Contains(got, "Learning #9 saved") {
		t.Errorf("legacy payload not rendered as saved: %q", got)
	}
}
