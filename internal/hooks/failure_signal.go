package hooks

import (
	"github.com/carsteneu/yesmem/internal/storage"
	"github.com/carsteneu/yesmem/internal/textutil"
)

// nearDuplicateSimilarity and nearDuplicateRatio mirror the guard in
// internal/daemon/handler_learnings.go: two texts state the same thing when the
// smaller one is contained in the larger *and* both are comparable in size. A
// fragment of a much longer entry is not a restatement.
const (
	nearDuplicateSimilarity = 0.5
	nearDuplicateRatio      = 0.6

	// candidateWindow bounds the scan to the most recent gotchas, the same way
	// the daemon handler bounds its own scan. A near repeat outside the window is
	// simply not counted — the failure itself is stored either way.
	candidateWindow = 500
)

// reinforceRecurringFailure bumps fail_count of an existing gotcha that states
// the same failure in slightly different words.
//
// The failure itself is stored as its own row — the error text is the
// information, and hash-only dedup no longer absorbs it. But the hard-block
// escalation in check.go counts fail_count (blockThreshold), and with
// byte-identical dedup alone it would only ever fire on exactly repeated errors.
// A near repeat is still a repeat: it counts towards that escalation without
// touching the older row's content, match_count or anything else.
func reinforceRecurringFailure(store *storage.Store, content string) {
	existing, err := store.GetActiveLearnings("gotcha", "", "", "", candidateWindow)
	if err != nil {
		return
	}
	newTokens := textutil.Tokenize(content)
	for i := range existing {
		tokens := textutil.Tokenize(existing[i].Content)
		if textutil.TokenSimilarity(newTokens, tokens) < nearDuplicateSimilarity {
			continue
		}
		if textutil.TokenLengthRatio(newTokens, tokens) < nearDuplicateRatio {
			continue
		}
		store.IncrementFailCounts([]int64{existing[i].ID})
		return
	}
}
