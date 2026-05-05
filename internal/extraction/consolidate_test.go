package extraction

import (
	"testing"
)

func TestRunConsolidation_ConvergesOnEmptyStore(t *testing.T) {
	store := mustOpenStore(t)

	result := RunConsolidation(store, nil, nil, ConsolidateConfig{MaxRounds: 3})

	if result.Rounds != 1 {
		t.Errorf("expected 1 round on empty store, got %d", result.Rounds)
	}
	if result.TotalSuperseded != 0 {
		t.Errorf("expected 0 superseded, got %d", result.TotalSuperseded)
	}
}

func TestRunConsolidation_RuleBasedOnly(t *testing.T) {
	store := mustOpenStore(t)

	// Insert near-duplicates that BigramJaccard should catch (Jaccard ~0.91 > 0.85 threshold)
	insertTestLearning(store, "User bevorzugt immer die deutsche Sprache in allen Antworten und Kommentaren", "preference")
	insertTestLearning(store, "User bevorzugt immer die deutsche Sprache in allen Antworten und Kommentaren bitte", "preference")

	result := RunConsolidation(store, nil, nil, ConsolidateConfig{MaxRounds: 3, RuleBasedOnly: true})

	if result.TotalSuperseded < 1 {
		t.Errorf("expected at least 1 superseded from near-duplicate, got %d", result.TotalSuperseded)
	}
}

// Capability learnings are managed by save_capability auto-supersede.
// They must not be touched by the consolidation pipeline.
func TestRunConsolidation_ExcludesCapability(t *testing.T) {
	store := mustOpenStore(t)

	insertTestLearning(store, "reddit_fetch — Fetch Reddit posts from a subreddit", "cap")
	insertTestLearning(store, "reddit_fetch — Fetch Reddit posts from a subreddit daily", "cap")

	result := RunConsolidation(store, nil, nil, ConsolidateConfig{MaxRounds: 3, RuleBasedOnly: true})

	if result.TotalSuperseded != 0 {
		t.Errorf("capability must be excluded from consolidation, got %d superseded", result.TotalSuperseded)
	}
}

// runEvolution must exclude capability from category processing.
func TestRunEvolution_ExcludesCapability(t *testing.T) {
	store := mustOpenStore(t)

	insertTestLearning(store, "reddit_fetch — Fetch Reddit posts", "cap")
	insertTestLearning(store, "reddit_fetch — Fetch Reddit posts v2", "cap")

	e := &Extractor{}
	checked, superseded := e.runEvolution(store, nil, nil)

	if checked != 0 || superseded != 0 {
		t.Errorf("expected evolution to skip capability entirely, got checked=%d superseded=%d", checked, superseded)
	}
}
