package wikirender

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = false
		} else if !prev && b.Len() > 0 {
			b.WriteRune('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func fileSlug(path string) string {
	return strings.NewReplacer("/", "_").Replace(path)
}

func snippet(s string, n int) string {
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func badge(source string, quarantined bool) string {
	if quarantined {
		return "☣️"
	}
	switch source {
	case "user_stated":
		return "🟢"
	case "agreed_upon":
		return "🟡"
	case "explicit_teaching":
		return "🟠"
	case "claude_suggested":
		return "🔵"
	case "llm_extracted":
		return "🟣"
	}
	return "⚪"
}

func trustScore(source string, quarantined bool) string {
	if quarantined {
		return "0.0"
	}
	switch source {
	case "user_stated":
		return "1.0"
	case "agreed_upon":
		return "0.8"
	case "explicit_teaching":
		return "0.7"
	case "claude_suggested":
		return "0.5"
	case "llm_extracted":
		return "0.3"
	}
	return "0.0"
}

func daysAgo(ts string) string {
	if ts == "" {
		return "?"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%dd ago", int(time.Since(t).Hours()/24))
}

func trustMix(ls []Learning) string {
	counts := map[string]int{}
	for _, l := range ls {
		counts[l.Source]++
	}
	order := []string{"user_stated", "agreed_upon", "explicit_teaching", "claude_suggested", "llm_extracted"}
	emoji := map[string]string{
		"user_stated": "🟢", "agreed_upon": "🟡", "explicit_teaching": "🟠",
		"claude_suggested": "🔵", "llm_extracted": "🟣",
	}
	var b strings.Builder
	for _, k := range order {
		if counts[k] > 0 {
			fmt.Fprintf(&b, "%s %s: %d\n", emoji[k], k, counts[k])
		}
	}
	return b.String()
}

func categoryMix(ls []Learning) string {
	counts := map[string]int{}
	for _, l := range ls {
		counts[l.Category]++
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "- **%s:** %d\n", k, counts[k])
	}
	return b.String()
}

func spanFirst(ls []Learning) string {
	min := ""
	for _, l := range ls {
		if min == "" || l.CreatedAt < min {
			min = l.CreatedAt
		}
	}
	return min
}

func spanLast(ls []Learning) string {
	max := ""
	for _, l := range ls {
		if l.CreatedAt > max {
			max = l.CreatedAt
		}
	}
	return max
}

func tplFuncs() template.FuncMap {
	return template.FuncMap{
		"badge":      badge,
		"trustScore": trustScore,
		"slugify":    slugify,
		"fileSlug":   fileSlug,
		"snippet":    snippet,
		"daysAgo":    daysAgo,
		"trustMix":   trustMix,
		"categoryMix": categoryMix,
		"spanFirst":  spanFirst,
		"spanLast":   spanLast,
		"base":       func(path string) string { return filepath.Base(path) },
		"isQuarantined": func(l Learning) bool { return l.QuarantinedAt != "" },
		"top3": func(ents []string) string {
			if len(ents) > 3 {
				ents = ents[:3]
			}
			return strings.Join(ents, " ")
		},
	}
}
