package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var allowedReadPrefixRE = regexp.MustCompile(`(?i)^\s*(select|with)\b`)

// QueryReadOnly executes a read-only SQL query against yesmem.db. The query is
// validated to start with SELECT or WITH and rejected if it contains additional
// statements after a semicolon. When initialized against a file-backed
// database, the underlying connection is opened with mode=ro so the SQLite
// driver enforces the read-only contract regardless of the validator.
func (s *Store) QueryReadOnly(ctx context.Context, query string, args []any) ([]string, [][]any, error) {
	if err := validateReadOnlyQuery(query); err != nil {
		return nil, nil, err
	}
	if s.readOnlyDB == nil {
		return nil, nil, errors.New("read-only database not initialized")
	}
	rows, err := s.readOnlyDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, nil
}

func validateReadOnlyQuery(q string) error {
	stripped := stripSQLComments(q)
	trimmed := strings.TrimSpace(stripped)
	if trimmed == "" {
		return errors.New("query must not be empty")
	}
	if !allowedReadPrefixRE.MatchString(trimmed) {
		return fmt.Errorf("read-only query must start with SELECT or WITH; got %q", firstWord(trimmed))
	}
	if hasExtraStatement(trimmed) {
		return errors.New("multi-statement queries are not allowed; use a single SELECT or WITH")
	}
	return nil
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' {
			return s[:i]
		}
	}
	return s
}

// stripSQLComments removes -- line comments and /* ... */ block comments while
// leaving content inside single-quoted string literals untouched.
func stripSQLComments(q string) string {
	var b strings.Builder
	i := 0
	for i < len(q) {
		c := q[i]
		if c == '\'' {
			b.WriteByte(c)
			i++
			for i < len(q) {
				b.WriteByte(q[i])
				if q[i] == '\'' {
					i++
					if i < len(q) && q[i] == '\'' {
						b.WriteByte(q[i])
						i++
						continue
					}
					break
				}
				i++
			}
			continue
		}
		if c == '-' && i+1 < len(q) && q[i+1] == '-' {
			j := strings.IndexByte(q[i:], '\n')
			if j == -1 {
				return b.String()
			}
			i += j
			continue
		}
		if c == '/' && i+1 < len(q) && q[i+1] == '*' {
			j := strings.Index(q[i+2:], "*/")
			if j == -1 {
				return b.String()
			}
			i += 2 + j + 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// hasExtraStatement returns true if the SQL contains a semicolon followed by
// non-whitespace content (i.e. another statement). Semicolons inside string
// literals are ignored.
func hasExtraStatement(q string) bool {
	inSingle := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' {
			if inSingle && i+1 < len(q) && q[i+1] == '\'' {
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if inSingle {
			continue
		}
		if c == ';' {
			rest := strings.TrimSpace(q[i+1:])
			if rest != "" {
				return true
			}
			return false
		}
	}
	return false
}
