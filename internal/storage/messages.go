package storage

import (
	"fmt"
	"log"
	"strings"

	"github.com/carsteneu/yesmem/internal/models"
)

// InsertMessages inserts a batch of messages in a single transaction.
// Writes to messages.db and syncs FTS5 index.
func (s *Store) InsertMessages(msgs []models.Message) error {
	db := s.messagesWriteDB()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO messages
		(session_id, source_agent, role, message_type, content, content_blob, tool_name, file_path, timestamp, sequence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	ftsStmt, err := tx.Prepare(`INSERT INTO messages_fts(rowid, content) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare fts: %w", err)
	}
	defer ftsStmt.Close()

	for _, m := range msgs {
		sourceAgent := models.NormalizeSourceAgent(m.SourceAgent)
		// Copy thinking blob to content for FTS5 indexing
		content := m.Content
		if content == "" && len(m.ContentBlob) > 0 {
			content = string(m.ContentBlob)
		}
		res, err := stmt.Exec(m.SessionID, sourceAgent, m.Role, m.MessageType, content,
			m.ContentBlob, m.ToolName, m.FilePath, fmtTime(m.Timestamp), m.Sequence)
		if err != nil {
			return fmt.Errorf("insert message seq=%d: %w", m.Sequence, err)
		}
		if content != "" {
			id, _ := res.LastInsertId()
			if _, err := ftsStmt.Exec(id, content); err != nil {
				log.Printf("warn: messages_fts insert failed for id=%d: %v", id, err)
			}
		}
	}

	return tx.Commit()
}

// GetMessagesBySession returns all messages for a session, ordered by sequence.
func (s *Store) GetMessagesBySession(sessionID string) ([]models.Message, error) {
	rows, err := s.messagesReaderDB().Query(`SELECT id, session_id, COALESCE(source_agent, 'claude'), role, message_type, content,
		content_blob, tool_name, file_path, timestamp, sequence
		FROM messages WHERE session_id = ? ORDER BY sequence`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get messages for %s: %w", sessionID, err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// GetMessagesBySessionAndType returns messages filtered by type, with optional limit.
func (s *Store) GetMessagesBySessionAndType(sessionID, msgType string, limit ...int) ([]models.Message, error) {
	q := `SELECT id, session_id, COALESCE(source_agent, 'claude'), role, message_type, content,
		content_blob, tool_name, file_path, timestamp, sequence
		FROM messages WHERE session_id = ? AND message_type = ? ORDER BY sequence`
	var args []any
	args = append(args, sessionID, msgType)
	if len(limit) > 0 && limit[0] > 0 {
		q += ` LIMIT ?`
		args = append(args, limit[0])
	}
	rows, err := s.messagesReaderDB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get messages for %s type %s: %w", sessionID, msgType, err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// DeleteMessagesBySession removes all messages for a session (for re-indexing).
func (s *Store) DeleteMessagesBySession(sessionID string) error {
	db := s.messagesWriteDB()
	// Delete FTS5 entries first
	db.Exec(`DELETE FROM messages_fts WHERE rowid IN (SELECT id FROM messages WHERE session_id = ?)`, sessionID)
	_, err := db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID)
	return err
}

// MessageSearchResult holds a single FTS5 search hit.
type MessageSearchResult struct {
	ID          int64
	SessionID   string
	SourceAgent string
	Content     string
	MessageType string
	Timestamp   string
	Sequence    int
	Rank        float64
}

// sanitizeFTS5Query quotes each term to prevent FTS5 from interpreting
// hyphens as column operators (e.g. "Multi-Agent" → "Multi" NOT column "Agent").
func sanitizeFTS5Query(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}
	var quoted []string
	for _, w := range words {
		w = strings.ReplaceAll(w, "\"", "")
		w = strings.ReplaceAll(w, "'", "")
		w = strings.TrimSpace(w)
		if w == "" || w == "OR" || w == "AND" || w == "NOT" || w == "NEAR" {
			continue
		}
		quoted = append(quoted, "\""+w+"\"")
	}
	if len(quoted) == 0 {
		return ""
	}
	return strings.Join(quoted, " OR ")
}

// SearchMessages performs FTS5 full-text search over messages.
// Replaces Bleve-based search.
func (s *Store) SearchMessages(query string, limit int) ([]MessageSearchResult, error) {
	return s.SearchMessagesCtx(query, "", "", limit)
}

// SearchMessagesCtx is SearchMessages with optional inclusive lower bound (since)
// and exclusive upper bound (before). Empty strings disable the respective bound.
// ISO-8601 timestamps compare lexicographically, so "2026-04-28" matches everything
// from that day onward (inclusive) and "2026-04-29" excludes that day onward.
func (s *Store) SearchMessagesCtx(query, since, before string, limit int) ([]MessageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}
	sql := `
		SELECT m.id, m.session_id, COALESCE(m.source_agent, 'claude'), m.content, m.message_type, m.timestamp, COALESCE(m.sequence, 0), bm25(messages_fts) as rank
		FROM messages_fts
		JOIN messages m ON m.id = messages_fts.rowid
		WHERE messages_fts MATCH ?`
	args := []any{ftsQuery}
	if since != "" {
		sql += ` AND m.timestamp >= ?`
		args = append(args, since)
	}
	if before != "" {
		sql += ` AND m.timestamp < ?`
		args = append(args, before)
	}
	sql += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	rows, err := s.messagesReaderDB().Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var results []MessageSearchResult
	for rows.Next() {
		var r MessageSearchResult
		if err := rows.Scan(&r.ID, &r.SessionID, &r.SourceAgent, &r.Content, &r.MessageType, &r.Timestamp, &r.Sequence, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchMessagesDeep performs FTS5 search with optional type filtering for deep_search.
// Type filter is applied in SQL via IN-clause so narrow date windows dominated by
// excluded types (e.g. tool_use) cannot collapse the result set.
func (s *Store) SearchMessagesDeep(query string, includeThinking, includeCommands bool, limit int) ([]MessageSearchResult, error) {
	return s.SearchMessagesDeepCtx(query, includeThinking, includeCommands, "", "", limit)
}

// SearchMessagesDeepCtx is SearchMessagesDeep with optional inclusive lower bound (since)
// and exclusive upper bound (before). Empty strings disable the respective bound.
func (s *Store) SearchMessagesDeepCtx(query string, includeThinking, includeCommands bool, since, before string, limit int) ([]MessageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	allowed := []string{"text", "assistant", "user"}
	if includeThinking {
		allowed = append(allowed, "thinking")
	}
	if includeCommands {
		allowed = append(allowed, "tool_use", "tool_result", "bash_output")
	}

	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(allowed))
	placeholders = placeholders[:len(placeholders)-1]

	sql := `
		SELECT m.id, m.session_id, COALESCE(m.source_agent, 'claude'), m.content, m.message_type, m.timestamp, COALESCE(m.sequence, 0), bm25(messages_fts) as rank
		FROM messages_fts
		JOIN messages m ON m.id = messages_fts.rowid
		WHERE messages_fts MATCH ?
		  AND m.message_type IN (` + placeholders + `)`
	args := []any{ftsQuery}
	for _, t := range allowed {
		args = append(args, t)
	}
	if since != "" {
		sql += ` AND m.timestamp >= ?`
		args = append(args, since)
	}
	if before != "" {
		sql += ` AND m.timestamp < ?`
		args = append(args, before)
	}
	// Multiplier guards against handler-side post-filtering (project, excludeSession).
	sql += ` ORDER BY rank LIMIT ?`
	args = append(args, limit*20)

	rows, err := s.messagesReaderDB().Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("deep search messages: %w", err)
	}
	defer rows.Close()

	var results []MessageSearchResult
	for rows.Next() {
		var r MessageSearchResult
		if err := rows.Scan(&r.ID, &r.SessionID, &r.SourceAgent, &r.Content, &r.MessageType, &r.Timestamp, &r.Sequence, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
		if len(results) >= limit {
			break
		}
	}
	return results, rows.Err()
}

// MessageCount returns total message count in messages.db.
func (s *Store) MessageCount() int64 {
	var count int64
	s.messagesReaderDB().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	return count
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func scanMessages(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]models.Message, error) {
	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		var ts string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.SourceAgent, &m.Role, &m.MessageType, &m.Content,
			&m.ContentBlob, &m.ToolName, &m.FilePath, &ts, &m.Sequence); err != nil {
			return nil, err
		}
		m.Timestamp = parseTime(ts)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
