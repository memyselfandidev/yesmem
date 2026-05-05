package proxy

import (
	"encoding/json"
	"sync"
)

// EagerStubMemory remembers, per session (threadID), which tool_use_ids have
// already been stubbed by EagerStubToolResults. Once a tool_use_id is in this
// memory, the corresponding tool_result is stubbed deterministically on every
// subsequent call — independent of hasFollowingAssistant or frozenBoundary.
// That keeps prefix bytes byte-identical across turns, so the prompt cache hits.
type EagerStubMemory struct {
	mu        sync.RWMutex
	stubbed   map[string]map[string]bool
	persistFn PersistFunc
	loadFn    LoadFunc
	loaded    map[string]bool
}

func NewEagerStubMemory() *EagerStubMemory {
	return &EagerStubMemory{
		stubbed: make(map[string]map[string]bool),
		loaded:  make(map[string]bool),
	}
}

func (m *EagerStubMemory) SetPersistFunc(fn PersistFunc) { m.persistFn = fn }
func (m *EagerStubMemory) SetLoadFunc(fn LoadFunc)       { m.loadFn = fn }

func (m *EagerStubMemory) WasStubbed(threadID, toolUseID string) bool {
	if m == nil || threadID == "" || toolUseID == "" {
		return false
	}
	m.ensureLoaded(threadID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ids, ok := m.stubbed[threadID]; ok {
		return ids[toolUseID]
	}
	return false
}

func (m *EagerStubMemory) RecordStubbed(threadID, toolUseID string) {
	if m == nil || threadID == "" || toolUseID == "" {
		return
	}
	m.ensureLoaded(threadID)
	m.mu.Lock()
	if m.stubbed[threadID] == nil {
		m.stubbed[threadID] = make(map[string]bool)
	}
	if m.stubbed[threadID][toolUseID] {
		m.mu.Unlock()
		return
	}
	m.stubbed[threadID][toolUseID] = true
	m.mu.Unlock()
	m.persist(threadID)
}

func (m *EagerStubMemory) ensureLoaded(threadID string) {
	m.mu.RLock()
	already := m.loaded[threadID]
	m.mu.RUnlock()
	if already {
		return
	}

	if m.loadFn == nil {
		m.mu.Lock()
		m.loaded[threadID] = true
		m.mu.Unlock()
		return
	}

	raw, ok := m.loadFn("eagerstub:" + threadID)
	if !ok || raw == "" {
		m.mu.Lock()
		m.loaded[threadID] = true
		m.mu.Unlock()
		return
	}

	var payload struct {
		ToolUseIDs []string `json:"tool_use_ids"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		m.mu.Lock()
		if m.stubbed[threadID] == nil {
			m.stubbed[threadID] = make(map[string]bool)
		}
		for _, id := range payload.ToolUseIDs {
			m.stubbed[threadID][id] = true
		}
		m.loaded[threadID] = true
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.loaded[threadID] = true
	m.mu.Unlock()
}

func (m *EagerStubMemory) persist(threadID string) {
	if m.persistFn == nil {
		return
	}
	m.mu.RLock()
	ids := make([]string, 0, len(m.stubbed[threadID]))
	for id := range m.stubbed[threadID] {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	payload := struct {
		ToolUseIDs []string `json:"tool_use_ids"`
	}{ToolUseIDs: ids}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	m.persistFn("eagerstub:"+threadID, string(data))
}

type EagerStubOption func(*eagerStubConfig)

type eagerStubConfig struct {
	memory      *EagerStubMemory
	threadID    string
	stickyHits  *int
	freshStubs  *int
}

func WithStubMemory(memory *EagerStubMemory, threadID string) EagerStubOption {
	return func(c *eagerStubConfig) {
		c.memory = memory
		c.threadID = threadID
	}
}

// WithStubCounters captures, for the duration of one EagerStubToolResults call,
// how many tool_results were stubbed via memory hit (sticky) vs. via a fresh
// in-call decision. Both pointers must be non-nil.
func WithStubCounters(sticky, fresh *int) EagerStubOption {
	return func(c *eagerStubConfig) {
		c.stickyHits = sticky
		c.freshStubs = fresh
	}
}
