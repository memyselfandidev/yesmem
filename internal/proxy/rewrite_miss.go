package proxy

import (
	"log"
	"regexp"
	"strings"
	"time"
)

const rewriteMissLogInterval = time.Hour

var claudeCodeVersionRE = regexp.MustCompile(`(?i)(?:@anthropic-ai/)?claude-(?:code|cli)/([0-9][^\s;,)]+)`)

func extractClaudeCodeVersion(userAgent string) string {
	match := claudeCodeVersionRE.FindStringSubmatch(userAgent)
	if len(match) != 2 {
		return "unknown"
	}
	return strings.TrimRight(match[1], ".")
}

func (s *Server) shouldLogRewriteMiss(funcName, userAgent string, now time.Time) (string, bool) {
	version := extractClaudeCodeVersion(userAgent)
	key := funcName + "|" + version

	s.rewriteMissMu.Lock()
	defer s.rewriteMissMu.Unlock()

	if s.rewriteMissLog == nil {
		s.rewriteMissLog = make(map[string]time.Time)
	}
	if last, ok := s.rewriteMissLog[key]; ok && now.Sub(last) < rewriteMissLogInterval {
		return version, false
	}
	s.rewriteMissLog[key] = now
	return version, true
}

func (s *Server) logRewriteMiss(funcName, userAgent string) {
	version, ok := s.shouldLogRewriteMiss(funcName, userAgent, time.Now())
	if !ok {
		return
	}

	logger := s.logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf("[prompt-rewrite] miss func=%s claude_code_version=%s user_agent=%q", funcName, version, userAgent)
}
