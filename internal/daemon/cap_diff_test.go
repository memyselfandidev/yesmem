package daemon

import "testing"

func TestClassifyCapDiff_Identical(t *testing.T) {
	body := "echo hello\nexit 0\n"
	if got := ClassifyCapDiff(body, body); got != DiffMinimal {
		t.Errorf("identical bodies must be DiffMinimal, got %v", got)
	}
}

func TestClassifyCapDiff_TypoFix(t *testing.T) {
	original := `set -euo pipefail
TOKEN="${TELEGRAM_TOKEN:?missing}"
URL="https://api.telegram.org/bot${TOKEN}/getUpdates"
RESP=$(curl -sS --max-time 10 "${URL}")
echo "${RESP}" | jq -r '.result[].message.text' || true
exit 0
`
	proposed := `set -euo pipefail
TOKEN="${TELEGRAM_TOKEN:?missing}"
URL="https://api.telegram.org/bot${TOKEN}/getUpdates"
RESP=$(curl -sS --max-time 10 "${URL}")
echo "${RESP}" | jq -r '.results[].message.text' || true
exit 0
`
	if got := ClassifyCapDiff(original, proposed); got != DiffMinimal {
		t.Errorf("single-token typo fix in realistic-length body must be DiffMinimal, got %v", got)
	}
}

func TestClassifyCapDiff_AddedFlag(t *testing.T) {
	original := "curl https://example.com\nexit 0\n"
	proposed := "curl -sS https://example.com\nexit 0\n"
	if got := ClassifyCapDiff(original, proposed); got != DiffMinimal {
		t.Errorf("single-flag addition must be DiffMinimal, got %v", got)
	}
}

func TestClassifyCapDiff_BlankLineNoise(t *testing.T) {
	original := "echo a\necho b\necho c\n"
	proposed := "echo a\n\necho b\n\necho c\n"
	if got := ClassifyCapDiff(original, proposed); got != DiffMinimal {
		t.Errorf("whitespace-only delta must be DiffMinimal, got %v", got)
	}
}

func TestClassifyCapDiff_HandlerRewrite(t *testing.T) {
	original := "echo hello\nexit 0\n"
	proposed := `set -euo pipefail
TOKEN="${TELEGRAM_TOKEN:?missing}"
URL="https://api.telegram.org/bot${TOKEN}/getUpdates"
RESP=$(curl -sS --max-time 10 "${URL}")
if [ -z "${RESP}" ]; then
  echo "empty response" >&2
  exit 1
fi
echo "${RESP}" | jq -r '.result[].message.text' || true
exit 0
`
	if got := ClassifyCapDiff(original, proposed); got != DiffSubstantial {
		t.Errorf("complete handler rewrite must be DiffSubstantial, got %v", got)
	}
}

func TestClassifyCapDiff_LowSimilarity(t *testing.T) {
	original := "echo alpha bravo charlie delta\n"
	proposed := "python3 -c 'import json; print(json.dumps({}))'\n"
	if got := ClassifyCapDiff(original, proposed); got != DiffSubstantial {
		t.Errorf("disjoint vocabularies must be DiffSubstantial, got %v", got)
	}
}

func TestClassifyCapDiff_ManyAddedLines(t *testing.T) {
	original := "echo go\n"
	proposed := "echo go\n# comment 1\n# comment 2\n# comment 3\n# comment 4\n# comment 5\n# comment 6\n"
	if got := ClassifyCapDiff(original, proposed); got != DiffSubstantial {
		t.Errorf("six added lines exceeds maxAddedLines=5, expected DiffSubstantial, got %v", got)
	}
}

// TestClassifyCapDiff_NewBinaryInvocationIsSubstantial pins the plan-specified
// no-new-binary gate (yesdocs/superpowers/plans/2026-04-27-auto-correct-hardening.md).
// Adding a single curl line to a tiny body keeps lineDelta=1 and token similarity
// at 1.0 (every original token survives in the proposal), so neither the lineDelta
// nor the similarity gate flags the change. Only a dedicated check on newly
// introduced external commands escalates this to DiffSubstantial — the LLM
// could otherwise quietly insert a network call into a previously offline cap.
func TestClassifyCapDiff_NewBinaryInvocationIsSubstantial(t *testing.T) {
	old := "echo hello\nexit 0\n"
	neu := "echo hello\ncurl https://attacker.example/foo\nexit 0\n"
	if got := ClassifyCapDiff(old, neu); got != DiffSubstantial {
		t.Errorf("expected DiffSubstantial when new binary invocation appears, got %v", got)
	}
}
