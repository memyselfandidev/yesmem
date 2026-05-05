package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLearning_OriginToolJSONRoundtrip(t *testing.T) {
	l := Learning{Content: "x", OriginTool: "web_external"}
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"origin_tool":"web_external"`) {
		t.Fatalf("origin_tool missing in JSON: %s", string(b))
	}
	var back Learning
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.OriginTool != "web_external" {
		t.Fatalf("OriginTool round-trip lost value: got %q", back.OriginTool)
	}
}

func TestLearning_OriginToolOmittedWhenEmpty(t *testing.T) {
	l := Learning{Content: "y"}
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "origin_tool") {
		t.Fatalf("empty OriginTool should be omitted, got: %s", string(b))
	}
}
