package hooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var navTools = map[string]bool{
	"grep": true, "egrep": true, "fgrep": true, "rg": true,
	"sed": true, "awk": true,
	"cat": true, "head": true, "tail": true,
}

var excludedPrefixes = []string{
	"/var/log/", "/var/cache/", "/tmp/", "/etc/",
	"/proc/", "/sys/", "/dev/",
}

var excludedExtensions = []string{".log", ".txt"}

// ParseNavCommand checks if a shell command is a code-navigation command
// targeting specific files. Returns the tool name, file paths, and whether
// it matches. Handles pipes by examining only the first command segment.
func ParseNavCommand(cmd string) (tool string, files []string, ok bool) {
	if idx := strings.Index(cmd, "|"); idx >= 0 {
		cmd = strings.TrimSpace(cmd[:idx])
	}

	tokens := tokenizeShell(cmd)
	if len(tokens) == 0 {
		return "", nil, false
	}

	base := filepath.Base(tokens[0])
	if !navTools[base] {
		return "", nil, false
	}
	tool = base

	var positional []string
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		if strings.HasPrefix(t, "-") {
			continue
		}
		positional = append(positional, t)
	}

	switch tool {
	case "grep", "egrep", "fgrep", "rg", "sed", "awk":
		if len(positional) < 2 {
			return "", nil, false
		}
		files = positional[1:]
	case "cat", "head", "tail":
		files = positional
	}

	if len(files) == 0 {
		return "", nil, false
	}

	var valid []string
	for _, f := range files {
		if !isExcludedPath(f) {
			valid = append(valid, f)
		}
	}
	if len(valid) == 0 {
		return "", nil, false
	}

	return tool, valid, true
}

func tokenizeShell(cmd string) []string {
	var tokens []string
	var cur strings.Builder
	var quote byte

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func isExcludedPath(path string) bool {
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	ext := filepath.Ext(path)
	for _, exc := range excludedExtensions {
		if ext == exc {
			return true
		}
	}
	return false
}

var (
	reShDouble = regexp.MustCompile(`sh\(\s*"([^"]+)"`)
	reShSingle = regexp.MustCompile(`sh\(\s*'([^']+)'`)
	reShBack   = regexp.MustCompile("sh\\(\\s*`([^`]+)`")
	reRgCall   = regexp.MustCompile(`(?:^|[^a-zA-Z_])rg\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']`)
	reCatCall  = regexp.MustCompile(`(?:^|[^a-zA-Z_])cat\(\s*["']([^"']+)["']`)
)

func ParseREPLNavCommands(code string) []string {
	var cmds []string
	for _, re := range []*regexp.Regexp{reShDouble, reShSingle, reShBack} {
		for _, m := range re.FindAllStringSubmatch(code, -1) {
			inner := m[1]
			if idx := strings.Index(inner, "&&"); idx >= 0 {
				inner = strings.TrimSpace(inner[idx+2:])
			}
			_, _, ok := ParseNavCommand(inner)
			if ok {
				cmds = append(cmds, inner)
			}
		}
	}
	for _, m := range reRgCall.FindAllStringSubmatch(code, -1) {
		cmds = append(cmds, fmt.Sprintf(`rg "%s" %s`, m[1], m[2]))
	}
	for _, m := range reCatCall.FindAllStringSubmatch(code, -1) {
		cmds = append(cmds, fmt.Sprintf("cat %s", m[1]))
	}
	return cmds
}

// SuggestYesmemTool returns a suggestion for which yesmem MCP tool to use
// instead of the given shell navigation tool.
func SuggestYesmemTool(navTool, filePath string) string {
	switch navTool {
	case "grep", "egrep", "fgrep", "rg":
		return "search_code_index(pattern) for symbol search, or search_code(pattern) for text grep with graph context"
	case "sed":
		return "get_code_snippet(file, start_line, end_line) for reading a line range"
	case "cat":
		return "get_file_symbols(file) for symbol overview, then get_code_snippet for targeted ranges"
	case "head", "tail":
		return "get_code_snippet(file, start_line, end_line) for reading a specific range"
	case "awk":
		return "get_code_snippet(file, start_line, end_line) or search_code(pattern) depending on use"
	}
	return ""
}

// CheckCodeNav is the entry point for code-navigation detection in the
// PreToolUse hook. Returns a block reason and whether to block the command.
// The isIndexed callback checks whether a file is in the codescan index.
func CheckCodeNav(cmd, cwd, project, sessionID string, isIndexed func(string, string) bool, dismissed bool) (reason string, block bool) {
	if dismissed {
		return "", false
	}

	tool, files, ok := ParseNavCommand(cmd)
	if !ok {
		return "", false
	}

	for _, f := range files {
		if isIndexed(project, f) {
			suggestion := SuggestYesmemTool(tool, f)
			reason = "yesmem has indexed this file. Use MCP tools instead of shell navigation:\n" +
				"  " + suggestion + "\n" +
				"To dismiss for this session: dismiss_code_nav(session_id=\"" + sessionID + "\")"
			return reason, true
		}
	}

	return "", false
}
