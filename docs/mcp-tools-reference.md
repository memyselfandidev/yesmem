## MCP Tools Reference (70 tools)

All tools registered in `internal/mcp/server.go:registerTools()`. Budget ceiling: `server_budget_test.go` enforces max 19K chars for tool descriptions. No deprecated tool name forwarding exists — superseded names (e.g. `schedule_create`) were removed, not aliased.

Large results (>30K chars) include `_meta.anthropic/maxResultSizeChars` for truncation detection.

### Search & Retrieval
| Tool | Parameters | Description |
|------|-----------|-------------|
| `search` | `query`, `project?`, `since?`, `before?`, `limit?` | Full-text search across sessions |
| `deep_search` | `query`, `include_thinking?`, `include_commands?`, `project?`, `since?`, `before?`, `limit?` | Deep search with full content and ±3 message context |
| `hybrid_search` | `query`, `project?`, `since?`, `before?`, `limit?` | Hybrid BM25 + vector search |
| `docs_search` | `query`, `source?`, `section?`, `since?`, `before?`, `exact?`, `limit?`, `doc_type?` | Search indexed documentation |

### Learning Management
| Tool | Parameters | Description |
|------|-----------|-------------|
| `remember` | `text`, `category?`, `project?`, `model?`, `source?`, `origin?`, `supersedes?`, `entities?`, `actions?`, `trigger?`, `anticipated_queries?`, `context?`, `domain?`, `task_type?` | Save a learning for future sessions |
| `get_learnings` | `id?`, `history?`, `category?`, `project?`, `since?`, `before?`, `limit?`, `task_type?` | Retrieve learnings by category or ID |
| `resolve` | `learning_id`, `reason?` | Resolve an unfinished task |
| `resolve_by_text` | `text`, `project?` | Find and resolve unfinished task by text |
| `quarantine_session` | `session_id` | Quarantine session — exclude learnings from search |
| `skip_indexing` | `session_id` | Skip indexing for this session |

### Learning Metadata
| Tool | Parameters | Description |
|------|-----------|-------------|
| `query_facts` | `entity?`, `action?`, `keyword?`, `domain?`, `project?`, `category?`, `limit?` | Search learning metadata by entity, action, or keyword |
| `relate_learnings` | `learning_id_a`, `learning_id_b`, `relation_type` | Set semantic edge between two learnings |

### Session & Context
| Tool | Parameters | Description |
|------|-----------|-------------|
| `get_session` | `session_id`, `mode?`, `offset?`, `limit?` | Load a session by mode |
| `get_compacted_stubs` | `session_id`, `from_idx?`, `to_idx?` | Get compacted stubs for a session |
| `expand_context` | `query?`, `message_range?` | Expand archived conversation parts |
| `get_project_profile` | `project` | Auto-generated project portrait |
| `related_to_file` | `path` | Which sessions touched this file? |
| `get_coverage` | `project` | Which files were edited in a project? |
| `list_projects` | — | List all projects with session counts |
| `project_summary` | `project`, `limit?` | Chronological project summary |

### Persona
| Tool | Parameters | Description |
|------|-----------|-------------|
| `set_persona` | `trait_key`, `value`, `dimension?` | Set persona trait |
| `get_persona` | — | Current persona profile |

### Self-Feedback
| Tool | Parameters | Description |
|------|-----------|-------------|
| `get_self_feedback` | `days?` | Recent corrections and feedback |

### Configuration
| Tool | Parameters | Description |
|------|-----------|-------------|
| `set_config` | `key`, `value`, `session_id?` | Set runtime config |
| `get_config` | `key`, `session_id?` | Read runtime config |
| `pin` | `content`, `scope?`, `project?` | Pin an instruction visible in every turn |
| `unpin` | `id`, `scope?` | Remove a pin by ID |
| `get_pins` | `project?` | List active pins |

### Agent Communication
| Tool | Parameters | Description |
|------|-----------|-------------|
| `send_to` | `target`, `content`, `msg_type?` | Send message to another session |
| `whoami` | `project?` | Get own session ID and agent metadata |
| `broadcast` | `content`, `project` | Send message to all sessions in a project |

### Documentation
| Tool | Parameters | Description |
|------|-----------|-------------|
| `ingest_docs` | `name`, `path`, `version?`, `project?`, `domain?`, `rules?`, `trigger_extensions?`, `doc_type?` | Import documentation (.md/.txt/.rst/.pdf) into knowledge base |
| `list_docs` | `project?` | List indexed documentation sources |
| `remove_docs` | `name`, `project?` | Remove a documentation source and its data |

### Plan Management
| Tool | Parameters | Description |
|------|-----------|-------------|
| `set_plan` | `plan`, `scope?` | Set the active plan (thread-scoped, survives proxy-collapse) |
| `update_plan` | `plan?`, `completed?`, `add?`, `remove?` | Update active plan |
| `get_plan` | — | Get active plan |
| `complete_plan` | — | Mark plan as completed |

### Agent Orchestration
| Tool | Parameters | Description |
|------|-----------|-------------|
| `spawn_agent` | `project`, `section`, `caller_session?`, `token_budget?`, `max_turns?`, `model?`, `work_dir?`, `backend?` | Spawn agent for project section |
| `relay_agent` | `to`, `content`, `project?`, `caller_session?` | Inject content into a running agent's terminal |
| `stop_agent` | `to`, `project?` | Stop an agent |
| `stop_all_agents` | `project` | Stop all agents in project |
| `resume_agent` | `to`, `project?` | Resume a stopped agent |
| `list_agents` | `project?` | List agents with status and PID |
| `get_agent` | `to`, `project?` | Get agent details |
| `update_agent_status` | `phase`, `id?` | Update agent's semantic work phase |

### Scratchpad
| Tool | Parameters | Description |
|------|-----------|-------------|
| `scratchpad_write` | `project`, `section`, `content` | Write a section to the shared scratchpad (upsert) |
| `scratchpad_read` | `project`, `section?` | Read scratchpad sections |
| `scratchpad_list` | `project?` | List scratchpad projects and sections |
| `scratchpad_delete` | `project`, `section?` | Delete a scratchpad section or project |

### Code Intelligence
| Tool | Parameters | Description |
|------|-----------|-------------|
| `search_code_index` | `pattern`, `project`, `kind?`, `file_pattern?`, `limit?` | Search code graph for symbols by name pattern |
| `search_code` | `pattern`, `project`, `file_pattern?`, `limit?` | Grep source files, enriched with graph context |
| `get_code_context` | `qualified_name`, `project`, `include_neighbors?` | Symbol details: signature, file, connected nodes |
| `get_code_snippet` | `project`, `qualified_name?`, `file?`, `start_line?`, `end_line?` | Full symbol body from source (func, var, const, type) |
| `get_file_symbols` | `file`, `project` | List all top-level symbols in a file with line numbers |
| `get_dependency_map` | `package`, `project`, `depth?` | Package import graph with cycle detection |
| `graph_traverse` | `from`, `project`, `direction?`, `edge_type?`, `depth?` | Trace call paths and dependencies from a node |
| `get_file_index` | `project`, `dir?` | List source files in a directory with learning/gotcha annotations |

### Capabilities
| Tool | Parameters | Description |
|------|-----------|-------------|
| `get_caps` | `project?`, `name?`, `tag?`, `limit?` | Load saved cap definitions |
| `save_cap` | `name`, `scripts`, `description?`, `tags?`, `project?`, `tested?`, `test_date?`, `auto_active?`, `actions?` | Save an executable cap (tool definition). Auto-supersedes existing caps with the same name |
| `cap_proposal_decide` | `id`, `decision`, `notes?` | Accept or reject an auto-correct cap proposal |
| `list_cap_proposals` | `status?`, `project?`, `limit?` | List cap-correction proposals |
| `register_caps` | `project?`, `tag?` | Generate JavaScript registerTool() code for saved caps |
| `activate_cap` | `name`, `project?` | Activate a saved cap for the current thread |
| `deactivate_cap` | `name` | Deactivate a cap for the current thread |
| `execute_cap` | `name`, `fn?`, `args?` | Execute a saved CAP handler sandboxed (bun/bash) |
| `cap_store` | `capability`, `action`, `table?`, `columns?`, `data?`, `where?`, `args?`, `limit?` | Capability database — namespaced tables for structured data. Actions: `create_table`, `upsert`, `query`, `delete`, `list_tables`, `claim_and_read` (atomic claim + read for polling). |

### Scheduling
| Tool | Parameters | Description |
|------|-----------|-------------|
| `schedule` | `action`, `name?`, `cron?`, `prompt?`, `enabled?`, `recurring?`, `id?`, `mode?`, `cap_name?`, `script_name?`, `auto_correct?`, `allowed_ports?`, `sandbox?`, `interval_seconds?`, `model?` | Create, update, list, or run scheduled jobs. Jobs persist across daemon restarts |

### Navigation Dismissal
| Tool | Parameters | Description |
|------|-----------|-------------|
| `dismiss_code_nav` | `session_id` | Dismiss code-navigation suggestion for this session |
| `dismiss_repl_pattern` | `project`, `shape_hash` | Dismiss a recorded REPL command pattern from cap-suggestion analysis |

---

### Auto-Injected Parameters

Every MCP tool call receives these parameters injected by `proxyCall`/`proxyCallFormat`/`proxyCallWithThreadID`:

| Parameter | Source | Description |
|-----------|--------|-------------|
| `_caller_pid` | `os.Getppid()` | Resolves session ID from daemon pidMap |
| `_session_id` | `CLAUDE_SESSION_ID` / `CODEX_THREAD_ID` / OpenCode env | Multi-agent session identity |
| `_source_agent` | env auto-detection | `claude`, `codex`, or `opencode` |
| `_client_model` | `ANTHROPIC_MODEL` / `MODEL` etc. | Current model name (in `proxyCall`/`proxyCallFormat`) |
| `_cwd` | parent process CWD | Current working directory (in `proxyCall`/`proxyCallFormat`) |

Sessions from Claude Code are unchanged (raw `CLAUDE_SESSION_ID`). Codex sessions get `codex:` prefix. OpenCode sessions get `opencode:` prefix.
