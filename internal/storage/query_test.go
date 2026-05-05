package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryReadOnly_Select(t *testing.T) {
	s := mustOpen(t)

	if _, err := s.DB().Exec(
		`INSERT INTO learnings (project, content, category, source, confidence, created_at, model_used) VALUES (?, ?, ?, ?, ?, datetime('now'), 'test')`,
		"qtest", "hello", "decision", "user_stated", 1.0,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cols, rows, err := s.QueryReadOnly(context.Background(),
		`SELECT id, content, category FROM learnings WHERE project = ?`,
		[]any{"qtest"},
	)
	if err != nil {
		t.Fatalf("QueryReadOnly: %v", err)
	}
	if want := []string{"id", "content", "category"}; len(cols) != 3 || cols[0] != want[0] || cols[1] != want[1] || cols[2] != want[2] {
		t.Errorf("columns: got %v want %v", cols, want)
	}
	if len(rows) != 1 {
		t.Fatalf("row count: got %d want 1", len(rows))
	}
	got := rows[0]
	if got[1] != "hello" || got[2] != "decision" {
		t.Errorf("row[0]: got %v want [_, hello, decision]", got)
	}
}

func TestQueryReadOnly_AcceptsWith(t *testing.T) {
	s := mustOpen(t)

	cols, rows, err := s.QueryReadOnly(context.Background(),
		`WITH x(n) AS (VALUES (1), (2), (3)) SELECT n FROM x`, nil,
	)
	if err != nil {
		t.Fatalf("WITH should succeed: %v", err)
	}
	if len(cols) != 1 || cols[0] != "n" {
		t.Errorf("columns: got %v", cols)
	}
	if len(rows) != 3 {
		t.Errorf("row count: got %d want 3", len(rows))
	}
}

func TestQueryReadOnly_RejectsWrites(t *testing.T) {
	s := mustOpen(t)

	cases := []string{
		"INSERT INTO learnings (project, content) VALUES ('x', 'y')",
		"UPDATE learnings SET content = 'z'",
		"DELETE FROM learnings",
		"DROP TABLE learnings",
		"CREATE TABLE evil (x INTEGER)",
		"ALTER TABLE learnings ADD COLUMN evil TEXT",
		"REPLACE INTO learnings (project) VALUES ('x')",
		"PRAGMA journal_mode = DELETE",
		"VACUUM",
		"REINDEX",
		"ATTACH DATABASE 'evil.db' AS evil",
		"  ",
		"",
	}
	for _, q := range cases {
		_, _, err := s.QueryReadOnly(context.Background(), q, nil)
		if err == nil {
			t.Errorf("expected error for query: %q", q)
		}
	}
}

func TestQueryReadOnly_RejectsMultipleStatements(t *testing.T) {
	s := mustOpen(t)

	cases := []string{
		"SELECT 1; DROP TABLE learnings",
		"SELECT 1;DROP TABLE learnings",
		"WITH x AS (SELECT 1) SELECT * FROM x; PRAGMA journal_mode = DELETE",
	}
	for _, q := range cases {
		_, _, err := s.QueryReadOnly(context.Background(), q, nil)
		if err == nil {
			t.Errorf("expected multi-statement rejection for: %q", q)
			continue
		}
		if !strings.Contains(strings.ToLower(err.Error()), "multi") &&
			!strings.Contains(strings.ToLower(err.Error()), "single") {
			t.Errorf("error message should mention multi-statement, got: %v", err)
		}
	}
}

func TestQueryReadOnly_AllowsTrailingSemicolon(t *testing.T) {
	s := mustOpen(t)

	cols, _, err := s.QueryReadOnly(context.Background(), "SELECT 1 AS x;", nil)
	if err != nil {
		t.Fatalf("trailing semicolon should be allowed: %v", err)
	}
	if len(cols) != 1 || cols[0] != "x" {
		t.Errorf("columns: got %v", cols)
	}
}

func TestQueryReadOnly_StripsCommentsBeforeValidating(t *testing.T) {
	s := mustOpen(t)

	cases := map[string]bool{
		"-- evil comment\nSELECT 1":                true,
		"/* block comment */ SELECT 1":             true,
		"-- DROP TABLE learnings\nSELECT 1":        true,
		"SELECT 1 -- DROP TABLE learnings":         true,
		"-- only comment":                          false,
		"-- comment\nDROP TABLE learnings":         false,
		"/* block */ DROP TABLE learnings":         false,
	}
	for q, ok := range cases {
		_, _, err := s.QueryReadOnly(context.Background(), q, nil)
		if ok && err != nil {
			t.Errorf("expected success for %q, got %v", q, err)
		}
		if !ok && err == nil {
			t.Errorf("expected rejection for %q", q)
		}
	}
}

func TestQueryReadOnly_DriverEnforcesReadOnly(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "yesmem.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if _, err := s.DB().Exec(
		`INSERT INTO learnings (project, content, category, source, confidence, created_at, model_used) VALUES (?, ?, ?, ?, ?, datetime('now'), 'test')`,
		"rotest", "before", "decision", "user_stated", 1.0,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	roDB := s.readOnlyDB
	if roDB == nil {
		t.Fatal("readOnlyDB not initialized")
	}
	if roDB == s.db {
		t.Fatal("on file-backed DB, readOnlyDB must be a separate connection from writer")
	}

	_, err = roDB.Exec(`UPDATE learnings SET content = 'after' WHERE project = 'rotest'`)
	if err == nil {
		t.Fatal("readOnlyDB should reject UPDATE")
	}

	var content string
	if err := s.DB().QueryRow(`SELECT content FROM learnings WHERE project = 'rotest'`).Scan(&content); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if content != "before" {
		t.Errorf("content was modified: got %q", content)
	}
}
