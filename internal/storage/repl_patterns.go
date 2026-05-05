package storage

import (
	"database/sql"
	"strings"
)

// tableReplPatternObservations tracks shape-hashes of shell commands run in
// the REPL, per project, with counts, suggestion-state, and dismissal-state.
// Two structurally similar commands (same NormalizeShellCommand output) share
// a shape_hash and collapse onto one row via UNIQUE(project, shape_hash).
// matched_cap is the cap-name inferred from cap_<name>__<table> references in
// the command, or "" if no active-cap match. Re-recordings backfill an empty
// matched_cap so the suggestion path can pick up later evidence.
const tableReplPatternObservations = `CREATE TABLE IF NOT EXISTS repl_pattern_observations (
	id                     INTEGER PRIMARY KEY AUTOINCREMENT,
	project                TEXT NOT NULL,
	shape_hash             TEXT NOT NULL,
	first_cmd_example      TEXT NOT NULL,
	matched_cap            TEXT NOT NULL DEFAULT '',
	count                  INTEGER NOT NULL DEFAULT 1,
	dismiss_count          INTEGER NOT NULL DEFAULT 0,
	first_seen             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	suggested_at           DATETIME,
	dismissed_permanent_at DATETIME,
	UNIQUE(project, shape_hash)
)`

// ReplPatternObservation is one row of repl_pattern_observations.
type ReplPatternObservation struct {
	ID                   int64
	Project              string
	ShapeHash            string
	FirstCmdExample      string
	MatchedCap           string
	Count                int
	DismissCount         int
	FirstSeen            string
	LastSeen             string
	SuggestedAt          sql.NullString
	DismissedPermanentAt sql.NullString
}

// RecordReplPattern inserts-or-increments. A row already marked
// dismissed_permanent_at is left untouched (no-op for that pattern).
// matchedCap may be "" if no cap_<name>__<table> reference was detected;
// on conflict, an existing empty matched_cap is backfilled with a new
// non-empty value (later evidence wins, never overwrites once set).
func (s *Store) RecordReplPattern(project, shapeHash, example, matchedCap string) error {
	_, err := s.db.Exec(`
		INSERT INTO repl_pattern_observations (project, shape_hash, first_cmd_example, matched_cap)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(project, shape_hash) DO UPDATE SET
			count = count + 1,
			last_seen = CURRENT_TIMESTAMP,
			matched_cap = CASE
				WHEN matched_cap = '' AND excluded.matched_cap != '' THEN excluded.matched_cap
				ELSE matched_cap
			END
		WHERE dismissed_permanent_at IS NULL`,
		project, shapeHash, example, matchedCap)
	return err
}

// GetReadyReplPatternSuggestion returns the top pattern for a project whose
// count is at least threshold, that hasn't been suggested yet, and hasn't been
// permanently dismissed. Atomically marks suggested_at=NOW on the returned row.
// Returns (nil, nil) if no pattern qualifies.
func (s *Store) GetReadyReplPatternSuggestion(project string, threshold int) (*ReplPatternObservation, error) {
	row := s.readerDB().QueryRow(`
		SELECT id, project, shape_hash, first_cmd_example, matched_cap, count, dismiss_count,
		       first_seen, last_seen, suggested_at, dismissed_permanent_at
		FROM repl_pattern_observations
		WHERE project = ?
		  AND count >= ?
		  AND suggested_at IS NULL
		  AND dismissed_permanent_at IS NULL
		ORDER BY count DESC, last_seen DESC
		LIMIT 1`,
		project, threshold)
	var p ReplPatternObservation
	err := row.Scan(&p.ID, &p.Project, &p.ShapeHash, &p.FirstCmdExample, &p.MatchedCap,
		&p.Count, &p.DismissCount, &p.FirstSeen, &p.LastSeen,
		&p.SuggestedAt, &p.DismissedPermanentAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`
		UPDATE repl_pattern_observations
		SET suggested_at = CURRENT_TIMESTAMP
		WHERE id = ?`, p.ID); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetReadyReplPatternSuggestionForActiveCaps is the active-cap-filtered
// variant of GetReadyReplPatternSuggestion. It only returns rows whose
// matched_cap is non-empty AND appears in activeCaps. An empty activeCaps
// list returns (nil, nil) without touching the DB. Atomically marks
// suggested_at=NOW on the returned row.
func (s *Store) GetReadyReplPatternSuggestionForActiveCaps(project string, threshold int, activeCaps []string) (*ReplPatternObservation, error) {
	if len(activeCaps) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(activeCaps))
	args := make([]any, 0, 2+len(activeCaps))
	args = append(args, project, threshold)
	for i, cap := range activeCaps {
		placeholders[i] = "?"
		args = append(args, cap)
	}
	query := `
		SELECT id, project, shape_hash, first_cmd_example, matched_cap, count, dismiss_count,
		       first_seen, last_seen, suggested_at, dismissed_permanent_at
		FROM repl_pattern_observations
		WHERE project = ?
		  AND count >= ?
		  AND matched_cap != ''
		  AND matched_cap IN (` + strings.Join(placeholders, ",") + `)
		  AND suggested_at IS NULL
		  AND dismissed_permanent_at IS NULL
		ORDER BY count DESC, last_seen DESC
		LIMIT 1`
	row := s.readerDB().QueryRow(query, args...)
	var p ReplPatternObservation
	err := row.Scan(&p.ID, &p.Project, &p.ShapeHash, &p.FirstCmdExample, &p.MatchedCap,
		&p.Count, &p.DismissCount, &p.FirstSeen, &p.LastSeen,
		&p.SuggestedAt, &p.DismissedPermanentAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`
		UPDATE repl_pattern_observations
		SET suggested_at = CURRENT_TIMESTAMP
		WHERE id = ?`, p.ID); err != nil {
		return nil, err
	}
	return &p, nil
}

// DismissReplPattern registers a user-dismissal: resets count to 0, clears
// suggested_at so it can be re-suggested after more records, and increments
// dismiss_count. When dismiss_count reaches 3, sets dismissed_permanent_at
// and the pattern becomes permanently ignored (future records no-op).
func (s *Store) DismissReplPattern(project, shapeHash string) error {
	_, err := s.db.Exec(`
		UPDATE repl_pattern_observations
		SET count = 0,
		    dismiss_count = dismiss_count + 1,
		    suggested_at = NULL,
		    dismissed_permanent_at = CASE
		        WHEN dismiss_count + 1 >= 3 THEN CURRENT_TIMESTAMP
		        ELSE dismissed_permanent_at
		    END
		WHERE project = ? AND shape_hash = ?`,
		project, shapeHash)
	return err
}
