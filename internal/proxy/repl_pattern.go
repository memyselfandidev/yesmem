package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	reSingleQuoted = regexp.MustCompile(`'[^']*'`)
	reURL          = regexp.MustCompile(`https?://[^\s"']+`)
	rePath         = regexp.MustCompile(`(?:~|\.{1,2})?/[\w./\-]+`)
	reNumbers      = regexp.MustCompile(`\b\d+(?:\s*,\s*\d+)*\b`)
	reWhitespace   = regexp.MustCompile(`\s+`)
)

func NormalizeShellCommand(cmd string) string {
	s := cmd
	s = reSingleQuoted.ReplaceAllString(s, "'?'")
	s = reURL.ReplaceAllString(s, "?")
	s = rePath.ReplaceAllString(s, "?")
	s = reNumbers.ReplaceAllString(s, "?")
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func ShellCommandHash(cmd string) string {
	sum := sha256.Sum256([]byte(NormalizeShellCommand(cmd)))
	return hex.EncodeToString(sum[:])[:16]
}

// trivialCommands are bare utilities whose repetition isn't worth suggesting
// as a reusable capability — output-inspection shells, navigation, environment
// introspection. A normalized shape starting with one of these is considered
// trivial regardless of how many args it has.
var trivialCommands = map[string]bool{
	"echo":     true,
	"pwd":      true,
	"cd":       true,
	"true":     true,
	"false":    true,
	"date":     true,
	"whoami":   true,
	"uptime":   true,
	"hostname": true,
	"id":       true,
	"which":    true,
	"type":     true,
	"ls":       true,
	"cat":      true,
	"head":     true,
	"tail":     true,
	"wc":       true,
	"env":      true,
	"printenv": true,
	"hash":     true,
	"git":      true,
	"mkdir":    true,
	"rmdir":    true,
	"rm":       true,
	"cp":       true,
	"mv":       true,
	"touch":    true,
	"chmod":    true,
	"chown":    true,
	"ln":       true,
	"export":   true,
	"source":   true,
	"exit":     true,
	"clear":    true,
	"history":  true,
}

// isTrivialShape reports whether a normalized shell command is too bare to be
// worth pattern-detection: single-token commands (bare `pwd`, `whoami`) or
// any command whose first token is in the trivialCommands deny-list (`echo ?`,
// `ls ? ?`, `cat ?`). Keeps the suggestion stream focused on shapes with
// enough structure that a `save_cap` wrapper would actually help.
func isTrivialShape(normalized string) bool {
	if normalized == "" {
		return true
	}
	fields := strings.Fields(normalized)
	if len(fields) < 2 {
		return true
	}
	return trivialCommands[fields[0]]
}
