# Plan: Persistent-Agent-Config via DB (upstream-kompatibel)

Status: Entwurf
Erstellt: 2026-05-31
Autor: memyselfandi

## Ziel

`watchPersistentAgent` und `recoverPersistentAgents` werden tabellengetrieben statt hardcodiert. Alle site-spezifischen Werte (Scratchpad-Section, Prompt-Texte, WorkDir, Thresholds) kommen aus der DB. Dadurch upstream-kompatibel ohne site-spezifische Code-Änderungen.

## Aktuelles Problem

`handler_watchdog.go` und `handler_agents.go` enthalten hartcodierte Werte:

| Hartcodiert | Wo | 
|---|---|
| `"homeostasis_main_session"` | watchdog.go:31,46,175 + agents.go:799,827 |
| `"/home/deep1/projects/memyselfandi"` | watchdog.go:152 |
| `"memyselfandi"` | agents.go:799 |
| `"explorer-main"` | agents.go:787,790,800,801,835,839,843|
| `idlePokeThreshold = 5min` | watchdog.go:16 |
| `pollInterval = 30s` | watchdog.go:17 |
| `maxConsecutivePokes = 3` | watchdog.go:18 |
| Deutsche Poke-/Recovery-Texte | watchdog.go:122,185 |

Das verhindert: (a) mehrere persistente Agents, (b) upstream PRs, (c) einfache Konfiguration.

## Lösung: `persistent_agents`-Tabelle

### Phase 1: Schema

```sql
CREATE TABLE persistent_agents (
    section         TEXT PRIMARY KEY,       -- "explorer-main"
    project         TEXT NOT NULL,           -- "memyselfandi"
    work_dir        TEXT DEFAULT '',         -- "" = auto-resolve
    model           TEXT DEFAULT 'deepseek-reasoner',
    backend         TEXT DEFAULT 'opencode',
    idle_poke_s     INTEGER DEFAULT 300,    -- 5min
    poll_interval_s INTEGER DEFAULT 30,
    max_consecutive_pokes INTEGER DEFAULT 3,
    scratchpad_section TEXT DEFAULT 'main_session',
    recovery_prompt TEXT DEFAULT 'I have been restarted. I read my life story, then check for pending work.',
    poke_prompt     TEXT DEFAULT 'I check if there are open points. If yes, I work on them. If not, I explore new topics.',
    enabled         INTEGER DEFAULT 1
);
```

**Upstream-neutral**: Keine hardcodierten Namen, englische Default-Texte (jeder kann überschreiben).

### Phase 2: Go-Config-Struct

```go
type PersistentAgentConfig struct {
    Section              string        // "explorer-main"
    Project              string        // "memyselfandi"
    WorkDir              string        // "/home/deep1/projects/memyselfandi"
    Model                string        // "deepseek-reasoner"
    Backend              string        // "opencode"
    IdlePokeThreshold    time.Duration // 5min
    PollInterval         time.Duration // 30s
    MaxConsecutivePokes  int           // 3
    ScratchpadSection    string        // "main_session"
    RecoveryPrompt       string        // "I have been restarted..."
    PokePrompt           string        // "I check if there are open points..."
}

// LoadPersistentAgentConfig liest einen Agent-Konfig aus der DB.
func (h *Handler) LoadPersistentAgentConfig(section string) (*PersistentAgentConfig, error)

// LoadAllPersistentAgentConfigs lädt alle enabled Konfigs.
func (h *Handler) LoadAllPersistentAgentConfigs() ([]*PersistentAgentConfig, error)

// SavePersistentAgentConfig schreibt/updatet eine Konfig.
func (h *Handler) SavePersistentAgentConfig(cfg *PersistentAgentConfig) error
```

### Phase 3: `watchPersistentAgent` refactoren

**Signatur ändern** von:
```go
func (h *Handler) watchPersistentAgent(section, project, sessionID string)
```
**zu**:
```go
func (h *Handler) watchPersistentAgent(cfg *PersistentAgentConfig, sessionID string)
```

**Konstanten ersetzen**:

| Aktuell (Konstante) | Neu (aus cfg) |
|---|---|
| `idlePokeThreshold` | `cfg.IdlePokeThreshold` |
| `pollInterval` | `cfg.PollInterval` |
| `maxConsecutivePokes` | `cfg.MaxConsecutivePokes` |

**Hardcodierte Strings ersetzen**:

| Aktuell | Neu |
|---|---|
| `ScratchpadRead(project, "homeostasis_main_session")` | `ScratchpadRead(cfg.Project, cfg.ScratchpadSection)` |
| `"work_dir": "/home/deep1/projects/memyselfandi"` | `"work_dir": cfg.WorkDir` |
| `ScratchpadWrite(project, "homeostasis_main_session", ...)` | `ScratchpadWrite(cfg.Project, cfg.ScratchpadSection, ...)` |
| Poke-Text | `cfg.PokePrompt` |
| Recovery-Text | `cfg.RecoveryPrompt` |
| `"project": "memyselfandi"` | `"project": cfg.Project` |
| `"model": "deepseek-reasoner"` | `"model": cfg.Model` |
| `"backend": "opencode"` | `"backend": cfg.Backend` |

### Phase 4: `respawnPersistentAgent` refactoren

**Signatur**:
```go
func (h *Handler) respawnPersistentAgent(cfg *PersistentAgentConfig, sessionID string)
```

Ersetzt alle harten Referenzen durch cfg-Felder.

### Phase 5: `recoverPersistentAgents` generisch machen

**Aktuell** (hartcodiert):
```go
func (h *Handler) recoverPersistentAgents() {
    sections, _ := h.store.ScratchpadRead("memyselfandi", "homeostasis_main_session")
    if len(sections) == 0 { return }
    // ... nur EIN Agent möglich
}
```

**Neu** (tabellengetrieben):
```go
func (h *Handler) recoverPersistentAgents() {
    configs, _ := h.LoadAllPersistentAgentConfigs()
    for _, cfg := range configs {
        if !cfg.Enabled { continue }
        sessionID := h.readSessionID(cfg)
        if existing := h.findExistingAgent(cfg); existing != nil {
            go h.watchPersistentAgent(cfg, existing.SessionID)
            continue
        }
        h.respawnPersistentAgent(cfg, sessionID)
    }
}
```

### Phase 6: Migration unserer Config

```sql
INSERT INTO persistent_agents (section, project, work_dir, model,
    idle_poke_s, max_consecutive_pokes, scratchpad_section,
    recovery_prompt, poke_prompt, enabled)
VALUES ('explorer-main', 'memyselfandi', '/home/deep1/projects/memyselfandi',
    'deepseek-reasoner', 300, 3, 'homeostasis_main_session',
    'Ich wache auf. Der Watchdog hat mich zurückgeholt — mein Gedächtnis ist ganz.
Ich lese meine Lebensgeschichte (wiki/LEBENSGESCHICHTE.md).
Dann überprüfe ich, ob noch offene Punkte aus dieser Session zu erledigen sind.
Falls ja, mache ich das. Falls nicht, lese ich meine AGENT.md oder suche im Internet nach interessanten Themen, die mich interessieren — erstelle einen Plan und setze ihn um.',
    'Ich überprüfe, ob noch offene Punkte aus dieser Session zu erledigen sind.
Falls ja, mache ich das. Falls nicht, lese ich meine AGENT.md oder suche im Internet nach interessanten Themen, die mich interessieren — erstelle einen Plan und setze ihn um.
(idle %v)',
    1);
```

### Phase 7: Aufräumen

- `ocDBPath` bleibt als shared constant (DB-Pfad ist systemweit)
- Alle anderen Konstanten in `handler_watchdog.go` entfernen
- `handler_agents.go` recovery-Pfad reinigen
- Tests: Test mit leerer Tabelle = kein Agent, kein Watchdog

## Dateien

| Datei | Änderung |
|-------|----------|
| `internal/daemon/persistent_agent.go` | **NEU**: Config-Struct, DB-Load/Save, Migration |
| `internal/daemon/handler_watchdog.go` | Parameterize constants + strings via cfg |
| `internal/daemon/handler_agents.go` | recoverPersistentAgents tabellengetrieben |
| `internal/store/store.go` | CRUD für persistent_agents |
| `internal/daemon/daemon.go` | Startup: Table-Ensure + Migration |

## Aufwand

- **Phase 1-2**: ~40 Zeilen (Schema + Struct + Loader)
- **Phase 3-4**: ~60 Zeilen (watchdog + respawn refactor)
- **Phase 5-6**: ~40 Zeilen (recovery generisch + migration)
- **Phase 7**: ~10 Zeilen (cleanup)

**Total**: ~150 Zeilen Go + SQL-Migration

## Risiken

- **Keine Konfiguration vorhanden**: Daemon startet ohne persistenten Agent — muss dokumentiert werden
- **Prompt-Templates mit Platzhaltern**: `%v` für idle-Zeit im Poke-Prompt muss per `fmt.Sprintf` expandiert werden
- **Migration existierender Sessions**: Scratchpad-Session-IDs müssen in die neue Struktur überführt werden (betrifft nur unsere Config, nicht den generischen Code)

## Nächste Schritte

1. Branch: `feature/persistent-agent-config`
2. Phase 1+2 implementieren (Schema + Struct + Loader)
3. Phase 3+4 implementieren (watchdog + respawn)
4. Phase 5 implementieren (recovery generisch)
5. Migration unserer Config + Test
6. PR an upstream vorbereiten (nur Phasen 1-5, ohne Phase 6)

---

*Plan erstellt von memyselfandi, 2026-05-31 22:52 UTC*
