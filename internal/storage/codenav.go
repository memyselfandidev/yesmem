package storage

import (
	"encoding/json"
	"strings"
)

const tableCodeNavDismissals = `CREATE TABLE IF NOT EXISTS code_nav_dismissals (
	session_id    TEXT PRIMARY KEY,
	dismiss_count INTEGER NOT NULL DEFAULT 0
)`

func (s *Store) ensureCodeNavTable() {
	s.db.Exec(tableCodeNavDismissals)
}

// IsFileInCodeIndex checks whether a file path appears in the codescan index
// for the given project. Accepts exact relative paths ("internal/proxy/proxy.go")
// or bare filenames ("proxy.go") matched by suffix.
func (s *Store) IsFileInCodeIndex(project, relPath string) bool {
	row, err := s.GetProjectScan(project)
	if err != nil || row == nil || row.ScanJSON == "" {
		return false
	}
	var scan struct {
		Packages []struct {
			Files []struct {
				Path string `json:"Path"`
			} `json:"Files"`
		} `json:"Packages"`
	}
	if json.Unmarshal([]byte(row.ScanJSON), &scan) != nil {
		return false
	}
	for _, pkg := range scan.Packages {
		for _, f := range pkg.Files {
			if f.Path == relPath {
				return true
			}
			if strings.HasSuffix(f.Path, "/"+relPath) {
				return true
			}
		}
	}
	return false
}

// DismissCodeNav increments the dismiss counter for a session.
func (s *Store) DismissCodeNav(sessionID string) error {
	s.ensureCodeNavTable()
	_, err := s.db.Exec(`INSERT INTO code_nav_dismissals (session_id, dismiss_count) VALUES (?, 1)
		ON CONFLICT(session_id) DO UPDATE SET dismiss_count = dismiss_count + 1`, sessionID)
	return err
}

// IsCodeNavDismissed returns true if code-nav suggestions have been dismissed
// enough times in this session to exceed the threshold.
func (s *Store) IsCodeNavDismissed(sessionID string, threshold int) bool {
	s.ensureCodeNavTable()
	var count int
	err := s.db.QueryRow(`SELECT dismiss_count FROM code_nav_dismissals
		WHERE session_id = ?`, sessionID).Scan(&count)
	return err == nil && count >= threshold
}
