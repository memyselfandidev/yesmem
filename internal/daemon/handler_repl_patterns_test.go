package daemon

import (
	"testing"
)

func TestHandleRecordReplPattern_OK(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleRecordReplPattern(map[string]any{
		"project":    "proj",
		"shape_hash": "h1",
		"example":    "sqlite3 db DELETE",
	})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestHandleRecordReplPattern_MissingProject(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleRecordReplPattern(map[string]any{"shape_hash": "h"})
	if resp.Error == "" {
		t.Error("expected error for missing project")
	}
}

func TestHandleRecordReplPattern_MissingShapeHash(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleRecordReplPattern(map[string]any{"project": "p"})
	if resp.Error == "" {
		t.Error("expected error for missing shape_hash")
	}
}

func TestHandleGetReplPatternSuggestion_BelowThreshold(t *testing.T) {
	h, _ := mustHandler(t)
	h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "c"})
	resp := h.handleGetReplPatternSuggestion(map[string]any{"project": "p", "threshold": float64(5)})
	if resp.Error != "" {
		t.Fatalf("err: %s", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Errorf("expected null result below threshold, got %s", string(resp.Result))
	}
}

func TestHandleGetReplPatternSuggestion_AtThreshold(t *testing.T) {
	h, _ := mustHandler(t)
	for i := 0; i < 5; i++ {
		h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "sqlite3 DELETE"})
	}
	resp := h.handleGetReplPatternSuggestion(map[string]any{"project": "p", "threshold": float64(5)})
	m := unmarshalMap(t, resp)
	pm, ok := m["pattern"].(map[string]any)
	if !ok {
		t.Fatalf("expected pattern key, got %v", m)
	}
	if pm["shape_hash"] != "h" {
		t.Errorf("shape_hash mismatch: %v", pm["shape_hash"])
	}
	if count, _ := pm["count"].(float64); count != 5 {
		t.Errorf("count: got %v, want 5", pm["count"])
	}
	if pm["first_cmd_example"] != "sqlite3 DELETE" {
		t.Errorf("first_cmd_example: %v", pm["first_cmd_example"])
	}
}

func TestHandleGetReplPatternSuggestion_DefaultsThresholdTo3(t *testing.T) {
	h, _ := mustHandler(t)
	for i := 0; i < 2; i++ {
		h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "c"})
	}
	resp := h.handleGetReplPatternSuggestion(map[string]any{"project": "p"})
	if string(resp.Result) != "null" {
		t.Errorf("expected null for count=2 (below default threshold=3), got %s", string(resp.Result))
	}
	h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "c"})
	resp = h.handleGetReplPatternSuggestion(map[string]any{"project": "p"})
	if string(resp.Result) == "null" {
		t.Errorf("expected pattern hit for count=3 (at default threshold=3), got null")
	}
}

func TestHandleGetReplPatternSuggestion_MissingProject(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleGetReplPatternSuggestion(map[string]any{})
	if resp.Error == "" {
		t.Error("expected error for missing project")
	}
}

func TestHandleGetReplPatternSuggestion_FiltersByActiveCaps(t *testing.T) {
	h, _ := mustHandler(t)
	for i := 0; i < 5; i++ {
		h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h-t", "example": "cap_telegram__updates", "matched_cap": "telegram"})
	}
	for i := 0; i < 3; i++ {
		h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h-r", "example": "cap_reddit__posts", "matched_cap": "reddit"})
	}
	resp := h.handleGetReplPatternSuggestion(map[string]any{
		"project":     "p",
		"threshold":   float64(3),
		"active_caps": []any{"reddit"},
	})
	if resp.Error != "" {
		t.Fatalf("err: %s", resp.Error)
	}
	m := unmarshalMap(t, resp)
	pm, ok := m["pattern"].(map[string]any)
	if !ok {
		t.Fatalf("expected pattern key, got %v", m)
	}
	if pm["matched_cap"] != "reddit" {
		t.Errorf("matched_cap: got %v, want reddit (telegram has higher count; only filter could surface reddit)", pm["matched_cap"])
	}
}

func TestHandleGetReplPatternSuggestion_ActiveCapsEmptyMeansNoSuggestion(t *testing.T) {
	h, _ := mustHandler(t)
	for i := 0; i < 3; i++ {
		h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "cap_reddit__posts", "matched_cap": "reddit"})
	}
	resp := h.handleGetReplPatternSuggestion(map[string]any{
		"project":     "p",
		"threshold":   float64(3),
		"active_caps": []any{},
	})
	if string(resp.Result) != "null" {
		t.Errorf("expected null with empty active_caps, got %s", string(resp.Result))
	}
}

func TestHandleDismissReplPattern_OK(t *testing.T) {
	h, _ := mustHandler(t)
	h.handleRecordReplPattern(map[string]any{"project": "p", "shape_hash": "h", "example": "c"})
	resp := h.handleDismissReplPattern(map[string]any{"project": "p", "shape_hash": "h"})
	if resp.Error != "" {
		t.Fatalf("dismiss: %s", resp.Error)
	}
}

func TestHandleDismissReplPattern_MissingParams(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleDismissReplPattern(map[string]any{"project": "p"})
	if resp.Error == "" {
		t.Error("expected error for missing shape_hash")
	}
}

func TestHandleDismissReplPattern_IdempotentOnNonexistent(t *testing.T) {
	h, _ := mustHandler(t)
	resp := h.handleDismissReplPattern(map[string]any{"project": "p", "shape_hash": "nonexistent"})
	if resp.Error != "" {
		t.Errorf("dismiss of nonexistent should be no-op success, got error: %s", resp.Error)
	}
}
