package textutil

import "strings"

// negationMarkers are tokens that flip the polarity of a statement. Tokenize()
// lowercases and splits on punctuation, so contractions arrive in their stem
// form ("doesn't" becomes "doesn", "isn't" becomes "isn"). The set is
// deliberately small: it has to recognise the correction case — a restatement of
// an existing claim that now says the opposite — and nothing beyond that. "can"
// is left out on purpose, because "can't" and "can" collapse to the same token
// and a marker on "can" would flag half of the English language.
var negationMarkers = map[string]struct{}{
	// German
	"nicht":   {},
	"nie":     {},
	"niemals": {},
	"ohne":    {},
	"kein":    {},
	"keine":   {},
	"keinen":  {},
	"keinem":  {},
	"keiner":  {},
	"keines":  {},
	// assertion-flipping adjectives: "the claim is wrong" against "the claim"
	"falsch": {},
	"unwahr": {},
	// English
	"not":       {},
	"no":        {},
	"never":     {},
	"without":   {},
	"none":      {},
	"false":     {},
	"wrong":     {},
	"incorrect": {},
	"cannot":    {},
	// tokenized contractions
	"doesn":   {},
	"isn":     {},
	"wasn":    {},
	"aren":    {},
	"didn":    {},
	"don":     {},
	"won":     {},
	"couldn":  {},
	"shouldn": {},
	"wouldn":  {},
}

// Negated reports whether the token stream carries an explicit negation.
func Negated(tokens []string) bool {
	for _, t := range tokens {
		if _, ok := negationMarkers[negationKey(t)]; ok {
			return true
		}
	}
	return false
}

// negationKey strips the punctuation Tokenize keeps inside a token ('.', '-',
// '_', '/') so that a sentence-final marker still matches: "nicht." negates
// exactly like "nicht", and that is the idiomatic form of a correction.
func negationKey(token string) string {
	return strings.TrimRight(token, ".-_/")
}

// IsContradiction reports whether two token streams state opposite things: they
// share at least one substantive token and only one of them is negated. Such a
// text is a correction of the other one, not a duplicate of it — the case that
// must never be swallowed by deduplication.
//
// The rule errs towards "contradiction" when a negation is incidental ("not
// only X but Y"). For a memory that is the safe direction: a stored duplicate
// costs a row, a swallowed correction costs the truth.
func IsContradiction(a, b []string) bool {
	if Negated(a) == Negated(b) {
		return false
	}
	return sharedSubstantive(a, b) > 0
}

// sharedSubstantive counts the tokens two streams have in common, ignoring the
// negation markers themselves — "not" alone must not make two unrelated
// sentences overlap.
func sharedSubstantive(a, b []string) int {
	set := make(map[string]struct{}, len(b))
	for _, t := range b {
		key := negationKey(t)
		if _, negated := negationMarkers[key]; negated {
			continue
		}
		set[key] = struct{}{}
	}
	shared := 0
	for _, t := range a {
		key := negationKey(t)
		if _, negated := negationMarkers[key]; negated {
			continue
		}
		if _, ok := set[key]; ok {
			shared++
		}
	}
	return shared
}
