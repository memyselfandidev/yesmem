package daemon

import (
	"encoding/json"
	"testing"
)

func TestHandleRemember_StoresExplicitOrigin(t *testing.T) {
	h, s := mustHandler(t)

	resp := h.Handle(Request{
		Method: "remember",
		Params: map[string]any{
			"text":     "Expliziter Origin-Wert soll gespeichert werden",
			"category": "decision",
			"project":  "yesmem",
			"origin":   "user_override",
		},
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	id, ok := result["id"].(float64)
	if !ok || id <= 0 {
		t.Fatalf("invalid id in response: %v", result["id"])
	}

	learning, err := s.GetLearning(int64(id))
	if err != nil {
		t.Fatalf("get learning: %v", err)
	}
	if learning.OriginTool != "user_override" {
		t.Fatalf("origin_tool: got %q, want %q", learning.OriginTool, "user_override")
	}
}

func TestHandleRemember_DefaultsOriginToUser(t *testing.T) {
	h, s := mustHandler(t)

	resp := h.Handle(Request{
		Method: "remember",
		Params: map[string]any{
			"text":     "Ohne Origin-Param soll user als Default gesetzt werden",
			"category": "decision",
			"project":  "yesmem",
		},
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	id, ok := result["id"].(float64)
	if !ok || id <= 0 {
		t.Fatalf("invalid id in response: %v", result["id"])
	}

	learning, err := s.GetLearning(int64(id))
	if err != nil {
		t.Fatalf("get learning: %v", err)
	}
	if learning.OriginTool != "user" {
		t.Fatalf("origin_tool: got %q, want %q", learning.OriginTool, "user")
	}
}
