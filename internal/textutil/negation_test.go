package textutil

import "testing"

func TestNegated(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"diff_scope ist nach dem Merge blind", false},
		{"diff_scope ist nach dem Merge nicht blind", true},
		{"kein Test ist ohne Beweis etwas wert", true},
		{"the merge gate is skipped on fridays", false},
		{"the merge gate is never skipped on fridays", true},
		{"the parser doesn't accept trailing commas", true},
		{"merging without a proof is guessing", true},
		{"merging with a proof is fine", false},
		// Tokenize keeps '.' inside a token, so a sentence-final marker arrives as
		// "nicht." / "not." — the idiomatic form of a correction.
		{"das funktioniert nach dem Merge nicht.", true},
		{"this does not.", true},
		{"", false},
	}

	for _, tc := range cases {
		if got := Negated(Tokenize(tc.text)); got != tc.want {
			t.Errorf("Negated(%q) = %v, want %v (tokens %v)", tc.text, got, tc.want, Tokenize(tc.text))
		}
	}
}

// TestIsContradiction pins the polarity rule that separates a correction from a
// duplicate: same topic, opposite polarity. The rule may only fire when the two
// texts actually share content, otherwise any negated sentence would contradict
// any other.
func TestIsContradiction(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "german correction adds nicht",
			a:    "diff_scope ist nach dem Merge blind",
			b:    "diff_scope ist nach dem Merge nicht blind",
			want: true,
		},
		{
			name: "sentence-final negation is a negation",
			a:    "Das funktioniert nach dem Merge.",
			b:    "Das funktioniert nach dem Merge nicht.",
			want: true,
		},
		{
			name: "english correction adds never",
			a:    "the merge gate is skipped on fridays",
			b:    "the merge gate is never skipped on fridays",
			want: true,
		},
		{
			name: "contraction is a negation",
			a:    "the parser is strict",
			b:    "the parser doesn't accept trailing commas",
			want: true,
		},
		{
			name: "restatement of the correction keeps the polarity",
			a:    "the check does not replace a proof",
			b:    "the check does not replace a proof now",
			want: false,
		},
		{
			name: "no negation on either side",
			a:    "deploy restarts the daemon",
			b:    "deploy restarts the daemon and the schedule",
			want: false,
		},
		{
			name: "negation but no shared content",
			a:    "nginx needs a reload after cert rotation",
			b:    "the merge gate is not a formality",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContradiction(Tokenize(tc.a), Tokenize(tc.b)); got != tc.want {
				t.Errorf("IsContradiction(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := IsContradiction(Tokenize(tc.b), Tokenize(tc.a)); got != tc.want {
				t.Errorf("IsContradiction is not symmetric for %q / %q: got %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
}
