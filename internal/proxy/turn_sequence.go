package proxy

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func ExtractToolTypes(msg map[string]interface{}) []string {
	role, _ := msg["role"].(string)
	if role != "assistant" {
		return nil
	}
	content, ok := msg["content"].([]interface{})
	if !ok {
		return nil
	}
	var tools []string
	for _, block := range content {
		m, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == "tool_use" {
			if name, _ := m["name"].(string); name != "" {
				tools = append(tools, name)
			}
		}
	}
	return tools
}

func ComputeTurnHash(toolNames []string) string {
	if len(toolNames) == 0 {
		return ""
	}
	var deduped []string
	for _, name := range toolNames {
		if len(deduped) == 0 || deduped[len(deduped)-1] != name {
			deduped = append(deduped, name)
		}
	}
	joined := strings.Join(deduped, "→")
	h := sha256.Sum256([]byte(joined))
	return fmt.Sprintf("%x", h[:8])
}

func computeTurnHashFromMessages(msgs []any) (hash string, toolNames string) {
	var lastAssistant map[string]any
	for i := len(msgs) - 1; i >= 0; i-- {
		m, ok := msgs[i].(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "user" {
			continue
		}
		if role == "assistant" {
			lastAssistant = m
			break
		}
	}
	if lastAssistant == nil {
		return "", ""
	}
	tools := ExtractToolTypes(lastAssistant)
	if len(tools) == 0 {
		return "", ""
	}
	var deduped []string
	for _, name := range tools {
		if len(deduped) == 0 || deduped[len(deduped)-1] != name {
			deduped = append(deduped, name)
		}
	}
	return ComputeTurnHash(tools), strings.Join(deduped, "→")
}
