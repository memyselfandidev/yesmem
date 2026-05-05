package codescan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func renderWikiLink(project string) string {
	wikiPath := fmt.Sprintf("~/.claude/yesmem/wiki/%s/", project)
	timestamp := "not yet rendered — will appear after first wiki-tick"
	if home, err := os.UserHomeDir(); err == nil {
		healthPath := filepath.Join(home, ".claude", "yesmem", "wiki", project, "health.md")
		if info, err := os.Stat(healthPath); err == nil {
			timestamp = fmt.Sprintf("last build: %s", info.ModTime().Format("15:04"))
		}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n**Full code map:** `%s` (%s)\n\n", wikiPath, timestamp))
	b.WriteString("> Browse `packages.md` for a package-level overview with file counts,\n")
	b.WriteString("> LOC, gotchas, and TODOs. `index.md` for the full file tree.\n")
	b.WriteString("> Per-file deep-dives: `files/<path>.md` (symbols, imports, co-edits,\n")
	b.WriteString("> learnings, sessions). Path encoding: `/` → `_`.\n")
	b.WriteString("> Falls back to `search_code_index` / `get_code_snippet` / raw grep.\n\n")
	return b.String()
}
