package textutil

import (
	"strings"
	"unicode"
)

// Tokenize splits text into lowercase tokens for similarity comparison.
func Tokenize(s string) []string {
	lower := strings.ToLower(s)
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '/' || r == '.' {
			return r
		}
		return ' '
	}, lower)
	words := strings.Fields(normalized)
	var tokens []string
	for _, w := range words {
		if len(w) > 1 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// TokenLengthRatio returns the ratio of the shorter token count to the longer
// one (1.0 for equal lengths, 0 for an empty side).
//
// It exists because TokenSimilarity is a containment measure: a six-token note
// scores 1.0 against any long row that happens to contain those six tokens. The
// ratio keeps a fragment from being treated as a restatement of a long text,
// which is the difference between "same statement, different words" and "a
// fragment of a much larger entry".
func TokenLengthRatio(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shorter, longer := len(a), len(b)
	if shorter > longer {
		shorter, longer = longer, shorter
	}
	return float64(shorter) / float64(longer)
}

// TokenSimilarity computes containment similarity (fraction of shorter set found in longer).
func TokenSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}
	longSet := make(map[string]bool, len(longer))
	for _, t := range longer {
		longSet[t] = true
	}
	shared := 0
	for _, t := range shorter {
		if longSet[t] {
			shared++
		}
	}
	return float64(shared) / float64(len(shorter))
}
