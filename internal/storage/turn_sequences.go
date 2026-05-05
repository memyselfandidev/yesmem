package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const tableThreadSequences = `CREATE TABLE IF NOT EXISTS thread_sequences (
	thread_id   TEXT PRIMARY KEY,
	project     TEXT NOT NULL,
	turn_hashes TEXT NOT NULL DEFAULT '[]',
	updated_at  INTEGER NOT NULL
)`

const maxTurnHashes = 20

type ThreadSequence struct {
	ThreadID   string
	Project    string
	TurnHashes string
	UpdatedAt  int64
}

type WorkflowSuggestion struct {
	SequenceHash string
	ExampleTools string
	Count        int
}

func (s *Store) ensureThreadSequences() {
	s.db.Exec(tableThreadSequences)
}

func (s *Store) RecordTurnHash(threadID, project, turnHash, exampleTools string) error {
	s.ensureThreadSequences()
	now := time.Now().Unix()

	var existing string
	err := s.db.QueryRow(`SELECT turn_hashes FROM thread_sequences WHERE thread_id=?`, threadID).Scan(&existing)
	if err != nil {
		existing = "[]"
	}

	var hashes []string
	if err := json.Unmarshal([]byte(existing), &hashes); err != nil {
		hashes = []string{}
	}
	hashes = append(hashes, turnHash)
	if len(hashes) > maxTurnHashes {
		hashes = hashes[len(hashes)-maxTurnHashes:]
	}

	data, _ := json.Marshal(hashes)
	_, err = s.db.Exec(`INSERT INTO thread_sequences (thread_id, project, turn_hashes, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(thread_id) DO UPDATE SET turn_hashes=excluded.turn_hashes, project=excluded.project, updated_at=excluded.updated_at`,
		threadID, project, string(data), now)
	return err
}

func (s *Store) GetThreadSequence(threadID string) (*ThreadSequence, error) {
	s.ensureThreadSequences()
	var seq ThreadSequence
	err := s.db.QueryRow(`SELECT thread_id, project, turn_hashes, updated_at FROM thread_sequences WHERE thread_id=?`, threadID).
		Scan(&seq.ThreadID, &seq.Project, &seq.TurnHashes, &seq.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &seq, nil
}

func (s *Store) GetWorkflowSuggestions(project string, minCount int) ([]WorkflowSuggestion, error) {
	s.ensureThreadSequences()
	rows, err := s.db.Query(`SELECT thread_id, turn_hashes FROM thread_sequences WHERE project=?`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subseqCounts := map[string]int{}
	subseqTools := map[string]string{}

	for rows.Next() {
		var threadID, raw string
		if err := rows.Scan(&threadID, &raw); err != nil {
			continue
		}
		var hashes []string
		if err := json.Unmarshal([]byte(raw), &hashes); err != nil {
			continue
		}
		seen := map[string]bool{}
		for i := 0; i+3 <= len(hashes); i++ {
			sub := strings.Join(hashes[i:i+3], "→")
			h := hashSubsequence(sub)
			if !seen[h] {
				seen[h] = true
				subseqCounts[h]++
				if _, ok := subseqTools[h]; !ok {
					subseqTools[h] = sub
				}
			}
		}
	}

	var suggestions []WorkflowSuggestion
	for h, count := range subseqCounts {
		if count >= minCount {
			suggestions = append(suggestions, WorkflowSuggestion{
				SequenceHash: h,
				ExampleTools: subseqTools[h],
				Count:        count,
			})
		}
	}
	return suggestions, nil
}

func hashSubsequence(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}
