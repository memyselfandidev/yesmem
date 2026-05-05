package storage

import (
	"time"
)

// SessionCap tracks which capabilities have been activated for a given
// Claude Code conversation thread. Activation is the lazy-load trigger that
// makes a saved capability tool-callable for the remainder of the session.
type SessionCap struct {
	ThreadID       string
	CapName string
	ActivatedAt    int64
	LastUsedAt     *int64
}

// ActivateCap records that the given capability is active for the
// thread. Idempotent: activating the same (thread, name) twice is a no-op.
func (s *Store) ActivateCap(threadID, name string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO session_active_caps
			(thread_id, cap_name, activated_at)
			VALUES (?, ?, ?)`,
		threadID, name, time.Now().Unix(),
	)
	return err
}

// DeactivateCap removes the activation row. The bool indicates whether
// a row was actually deleted (false for nonexistent activations).
func (s *Store) DeactivateCap(threadID, name string) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM session_active_caps
			WHERE thread_id = ? AND cap_name = ?`,
		threadID, name,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetSessionCaps returns the capabilities active for the given thread,
// ordered by activation time.
func (s *Store) GetSessionCaps(threadID string) ([]SessionCap, error) {
	rows, err := s.db.Query(
		`SELECT thread_id, cap_name, activated_at, last_used_at
			FROM session_active_caps
			WHERE thread_id = ?
			ORDER BY activated_at`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var caps []SessionCap
	for rows.Next() {
		var c SessionCap
		var lastUsed *int64
		if err := rows.Scan(&c.ThreadID, &c.CapName, &c.ActivatedAt, &lastUsed); err != nil {
			return nil, err
		}
		c.LastUsedAt = lastUsed
		caps = append(caps, c)
	}
	return caps, rows.Err()
}

// TouchCap updates last_used_at to now for the given (thread, name).
// No-op if the activation row does not exist.
func (s *Store) TouchCap(threadID, name string) error {
	_, err := s.db.Exec(
		`UPDATE session_active_caps
			SET last_used_at = ?
			WHERE thread_id = ? AND cap_name = ?`,
		time.Now().Unix(), threadID, name,
	)
	return err
}
