package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strconv"
	"testing"
)

func newFreezeStripTestServer() *Server {
	return &Server{
		logger:      log.New(io.Discard, "", 0),
		frozenStubs: NewFrozenStubs(),
	}
}

func freezeStripBuildMsgs() []any {
	embedded := map[string]any{
		"type":          "text",
		"text":          "embedded-cc",
		"cache_control": map[string]any{"type": "ephemeral", "ttl": "5m"},
	}
	plain := func(text string) map[string]any {
		return map[string]any{"type": "text", "text": text}
	}
	msg := func(role string, blocks ...any) any {
		return map[string]any{"role": role, "content": blocks}
	}
	return []any{
		msg("user", plain("turn0")),
		msg("assistant", plain("ack0")),
		msg("user", embedded),
		msg("assistant", plain("ack1")),
		msg("user", plain("turn2")),
	}
}

// The frozen prefix stored by the FREEZE path must contain no embedded
// cache_control. Otherwise subsequent RESTORE turns strip them and produce
// different outgoing bytes than the FREEZE turn, causing an Anthropic prompt
// cache miss on the first turn after a sawtooth-emergency freeze.
func TestFreezeStubsAndInjectBreakpoint_StoresCleanFrozenMessages(t *testing.T) {
	s := newFreezeStripTestServer()
	threadID := "freeze-strip-stores-clean"

	msgs := freezeStripBuildMsgs()
	frozenLen := len(msgs)
	boundary := msgs[frozenLen-1]
	req := map[string]any{"messages": deepCopyMessages(msgs)}

	s.freezeStubsAndInjectBreakpoint(req, threadID, frozenLen, boundary, 1000, 1000)

	frozen := s.frozenStubs.Get(threadID, msgs)
	if frozen == nil {
		t.Fatal("expected frozenStubs.Get to return result")
	}
	for i, m := range frozen.Messages {
		raw, _ := json.Marshal(m)
		if bytes.Contains(raw, []byte(`"cache_control"`)) {
			t.Errorf("frozen.Messages[%d] still contains cache_control: %s", i, raw)
		}
	}
}

// Bytes of the frozen-prefix region must be byte-identical between the
// FREEZE-path turn and any RESTORE-path turn so Anthropic can hit the cache.
func TestSawtoothFreezeRestoreSymmetry_ProducesIdenticalFrozenPrefixBytes(t *testing.T) {
	s := newFreezeStripTestServer()
	threadID := "freeze-restore-byte-identity"

	inputMsgs := freezeStripBuildMsgs()
	frozenLen := len(inputMsgs)
	boundary := inputMsgs[frozenLen-1]

	reqT1 := map[string]any{"messages": deepCopyMessages(inputMsgs)}
	s.freezeStubsAndInjectBreakpoint(reqT1, threadID, frozenLen, boundary, 1000, 1000)
	bytesT1, _ := json.Marshal(reqT1["messages"].([]any)[:frozenLen])

	frozen := s.frozenStubs.Get(threadID, inputMsgs)
	if frozen == nil {
		t.Fatal("expected frozen entry after freeze")
	}
	reqT2 := map[string]any{"messages": append([]any{}, frozen.Messages...)}
	StripMessagesCacheControl(reqT2, 0, frozenLen)
	InjectFrozenStubCacheBreakpoint(reqT2, frozenLen)
	bytesT2, _ := json.Marshal(reqT2["messages"].([]any)[:frozenLen])

	if !bytes.Equal(bytesT1, bytesT2) {
		t.Errorf("FREEZE != RESTORE for frozen prefix bytes:\nT1=%s\nT2=%s", bytesT1, bytesT2)
	}
}

// One-turn cache recovery: when the FREEZE-turn pipeline mutates
// req["messages"] AFTER freezeStubsAndInjectBreakpoint (timestamps stamping,
// directives injection, etc.), UpdateMessages must persist those
// post-pipeline bytes so that the RESTORE-1 turn loads the same bytes and
// produces a byte-identical frozen prefix on the wire. Without UpdateMessages
// the RESTORE-1 wire bytes diverge from the FREEZE-1 wire bytes by the size
// of the post-pipeline mutation, breaking Anthropic's prefix cache for two
// turns. This test simulates the production pipeline's [msg:N] timestamp
// stamping and verifies the frozen-prefix bytes match across turns once the
// snapshot is refreshed.
func TestFrozenStubsUpdateMessages_PersistsPostPipelineBytesForOneTurnRecovery(t *testing.T) {
	s := newFreezeStripTestServer()
	threadID := "freeze-update-1turn"

	inputMsgs := freezeStripBuildMsgs()
	frozenLen := len(inputMsgs)
	boundary := inputMsgs[frozenLen-1]

	// === FREEZE turn ===
	reqT1 := map[string]any{"messages": deepCopyMessages(inputMsgs)}
	s.freezeStubsAndInjectBreakpoint(reqT1, threadID, frozenLen, boundary, 1000, 1000)

	// Simulate post-freeze pipeline mutation: timestamps stage prefixes each
	// text block with "[msg:N] ". In production this happens between L1173
	// (freeze) and L1609 (EnforceCacheBreakpointLimit) on proxy.go.
	msgsT1 := reqT1["messages"].([]any)
	for i := 0; i < frozenLen; i++ {
		m, _ := msgsT1[i].(map[string]any)
		content, _ := m["content"].([]any)
		for _, block := range content {
			b, _ := block.(map[string]any)
			if b["type"] == "text" {
				b["text"] = "[msg:" + strconv.Itoa(100+i) + "] " + b["text"].(string)
			}
		}
	}

	// FIX: snapshot post-pipeline bytes back into the frozen store so the
	// RESTORE-1 turn loads the post-mutation state, not the pre-mutation
	// state.
	if !s.frozenStubs.UpdateMessages(threadID, msgsT1[:frozenLen]) {
		t.Fatal("UpdateMessages should succeed for existing entry with matching length")
	}

	bytesT1, _ := json.Marshal(msgsT1[:frozenLen])

	// === RESTORE-1 turn ===
	frozen := s.frozenStubs.Get(threadID, msgsT1)
	if frozen == nil {
		t.Fatal("expected frozen entry after FREEZE+UpdateMessages")
	}

	reqT2 := map[string]any{"messages": append([]any{}, frozen.Messages...)}
	StripMessagesCacheControl(reqT2, 0, frozenLen)
	InjectFrozenStubCacheBreakpoint(reqT2, frozenLen)
	// RESTORE pipeline timestamps stage offset-skips the frozen range
	// (matching production's offset filter), so no further mutation here.

	bytesT2, _ := json.Marshal(reqT2["messages"].([]any)[:frozenLen])

	if !bytes.Equal(bytesT1, bytesT2) {
		t.Errorf("FREEZE-SENT bytes != RESTORE-1-SENT bytes after UpdateMessages\nT1=%s\nT2=%s", bytesT1, bytesT2)
	}
}

// UpdateMessages must refuse to overwrite a frozen entry when the new slice
// length differs from the stored length — that mismatch indicates the
// frozen range is no longer intact (e.g. a second sawtooth fired in the
// same pipeline).
func TestFrozenStubsUpdateMessages_RejectsLengthMismatchAndMissingEntry(t *testing.T) {
	s := newFreezeStripTestServer()
	threadID := "freeze-update-reject"

	if s.frozenStubs.UpdateMessages(threadID, []any{}) {
		t.Error("UpdateMessages should return false for missing entry")
	}

	inputMsgs := freezeStripBuildMsgs()
	frozenLen := len(inputMsgs)
	boundary := inputMsgs[frozenLen-1]
	reqT1 := map[string]any{"messages": deepCopyMessages(inputMsgs)}
	s.freezeStubsAndInjectBreakpoint(reqT1, threadID, frozenLen, boundary, 1000, 1000)

	short := reqT1["messages"].([]any)[:frozenLen-1]
	if s.frozenStubs.UpdateMessages(threadID, short) {
		t.Error("UpdateMessages should return false for length mismatch")
	}

	if got := s.frozenStubs.LengthFor(threadID); got != frozenLen {
		t.Errorf("LengthFor: want %d, got %d", frozenLen, got)
	}
}

// FROZEN-SNAPSHOT must capture the frozen-prefix bytes BEFORE post-freeze
// pipeline stages prepend injected turns to req["messages"]. If the snapshot
// runs at end-of-pipeline (after injectBriefingTurn / injectCodeMapTurn),
// msgs[:frozenLen] contains the injected turns at the head and shifts the
// actual frozen conversation prefix out of the snapshot, breaking cache
// continuity for 5-9 turns after every emergency-sawtooth.
func TestFrozenStubsUpdateMessages_StablePrefixAcrossInjectPrepend(t *testing.T) {
	s := newFreezeStripTestServer()
	threadID := "freeze-update-inject-prepend"

	inputMsgs := freezeStripBuildMsgs()
	frozenLen := len(inputMsgs)
	boundary := inputMsgs[frozenLen-1]

	reqT1 := map[string]any{"messages": deepCopyMessages(inputMsgs)}
	s.freezeStubsAndInjectBreakpoint(reqT1, threadID, frozenLen, boundary, 1000, 1000)

	msgsT1 := reqT1["messages"].([]any)
	for i := 0; i < frozenLen; i++ {
		m, _ := msgsT1[i].(map[string]any)
		content, _ := m["content"].([]any)
		for _, block := range content {
			b, _ := block.(map[string]any)
			if b["type"] == "text" {
				b["text"] = "[ts:" + strconv.Itoa(i) + "] " + b["text"].(string)
			}
		}
	}

	preInjectBytes, _ := json.Marshal(msgsT1[:frozenLen])

	if !s.frozenStubs.UpdateMessages(threadID, msgsT1[:frozenLen]) {
		t.Fatal("UpdateMessages must succeed at pre-inject snapshot position")
	}

	briefingTurn := map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "<briefing>"}}}
	briefingAck := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "ack-briefing"}}}
	codemapTurn := map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "<codemap>"}}}
	codemapAck := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "ack-codemap"}}}
	reqT1["messages"] = append([]any{briefingTurn, briefingAck, codemapTurn, codemapAck}, msgsT1...)

	postInjectMsgs := reqT1["messages"].([]any)
	naivePostInjectBytes, _ := json.Marshal(postInjectMsgs[:frozenLen])
	if bytes.Equal(naivePostInjectBytes, preInjectBytes) {
		t.Fatal("test setup invalid: naive post-inject slice equals pre-inject slice")
	}

	frozen := s.frozenStubs.Get(threadID, msgsT1)
	if frozen == nil {
		t.Fatal("frozen entry must be retrievable using the pre-inject prefix")
	}
	storedBytes, _ := json.Marshal(frozen.Messages)
	if !bytes.Equal(storedBytes, preInjectBytes) {
		t.Errorf("snapshot misaligned\nstored=%s\nwant   =%s", storedBytes, preInjectBytes)
	}
	for i, m := range frozen.Messages {
		raw, _ := json.Marshal(m)
		if bytes.Contains(raw, []byte("<briefing>")) || bytes.Contains(raw, []byte("<codemap>")) {
			t.Errorf("frozen.Messages[%d] contains injected content: %s", i, raw)
		}
	}
}
