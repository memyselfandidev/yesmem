package models

import "testing"

func TestOriginMultiplier_Defaults(t *testing.T) {
	cases := []struct {
		origin string
		want   float64
	}{
		{"user", 1.0},
		{"file_read", 0.9},
		{"bash_command_input", 0.7},
		{"llm_extracted_session", 0.6},
		{"cap_reddit_fetch", 0.5},
		{"web_external", 0.4},
		{"", 0.8},
	}
	for _, c := range cases {
		got := OriginMultiplier(c.origin)
		if got != c.want {
			t.Errorf("OriginMultiplier(%q) = %.2f, want %.2f", c.origin, got, c.want)
		}
	}
}

func TestOriginMultiplier_UnknownPrefix(t *testing.T) {
	got := OriginMultiplier("cap_anything")
	if got != 0.5 {
		t.Errorf("OriginMultiplier(\"cap_anything\") = %.2f, want 0.5", got)
	}
}
