# Cap-builder MCP API reference

Authoritative shapes for the MCP tools that drive the cap-builder workflow. These supersede the inline examples in any older skill; the current skill body documents `handler_repl`/`handler_bash` as separate fields, which is the legacy shape — the v1.1 canonical shape uses a `scripts:` JSON array. All tools live under the `mcp__yesmem__` namespace.

Authority: cross-checked against `internal/daemon/handler_caps.go`, `handler_caps_helpers.go`, `handler_cap_proposal.go`, `internal/storage/cap_store.go`, and the live save_cap returns observed in stage-1.

---

## save_cap

Create or update a cap. Re-saving with the same `name` auto-supersedes the prior row and bumps the version.

**Required**:

| Field | Type | Notes |
|---|---|---|
| `name` | string | Must match `[a-z][a-z0-9_]{0,63}` — lowercase, underscores, no dashes. |
| `description` | string | One-line discovery bait. Generic descriptions make caps invisible to future Claude. |
| `scripts` | string | **Stringified JSON array** of script entries (see below). |

**Optional**:

| Field | Type | Default | Notes |
|---|---|---|---|
| `tags` | string | `""` | Comma-separated. Reuse existing tags (`web`, `fetch`, `cap_store`, `analysis`). |
| `tested` | bool | `false` | Set `true` only after running with real inputs. |
| `test_date` | string | `""` | ISO date (`2026-05-01`). Audit trail. |
| `auto_active` | bool | `false` | If `true`, proxy injects into every session. Use sparingly — every session pays the token cost. |
| `supersedes` | int | (auto by name) | Explicit prior id. Usually omit; same `name` auto-supersedes. |

**Legacy fields** (still accepted by `scriptsFromSaveCapParams`, but do not use in new code):

| Field | Replaced by |
|---|---|
| `handler_repl` | `scripts: '[{name, kind:"tool", runtime:"repl", body, schema}]'` |
| `handler_bash` | `scripts: '[{..., runtime:"bash", schema:"{...}"}]'` |
| top-level `schema` | per-script `schema:` inside the scripts array |

**`scripts` entry shape** (Cap-Spec v1.1):

```json
{
  "name": "<script_name>",
  "kind": "tool" | "handler",
  "runtime": "repl" | "bash",
  "body": "<full source as a string>",
  "schema": "<single-line JSON, required for kind:tool runtime:bash; forbidden for kind:handler>"
}
```

`kind: tool` + `runtime: repl` may omit `schema` — derived from the JS function signature. `kind: tool` + `runtime: bash` REQUIRES `schema` (use `"{\"type\":\"object\"}"` for no-arg). `kind: handler` MUST NOT have `schema`.

**Returns**:

```json
{"id": 59608, "name": "wiki_export", "superseded": 59607, "version": 46}
```

Where `superseded` is the prior learnings id (or null on first save), and `version` auto-increments past any existing chain endpoint.

**Common errors**:

| Error | Cause | Fix |
|---|---|---|
| `scripts is required (JSON array)` | Empty `scripts` and no legacy `handler_*` | Pass a stringified JSON array. |
| `scripts: invalid JSON: ...` | `scripts` value isn't valid JSON | `JSON.stringify(...)` before passing. |
| `scripts[N]: name is required` | Entry missing `name` | Each entry needs `name`. |
| `invalid name "X": must match [a-z][a-z0-9_]{0,63}` | Caps, dashes, or starts with digit | Rename. |
| Body persisted as literal `"PLACEHOLDER"` | `scripts` JSON >10 KB inline in tool_use call | Build the body in a REPL variable, call `await mcp__yesmem__save_cap({scripts: ...})`. See `gotchas.md`. |

**Minimal example**:

```javascript
const handler = async ({url}) => {
  const r = await sh(`curl -s --max-time 15 ${shQuote(url)}`, 20000);
  return {ok: true, bytes: r.length};
};

await mcp__yesmem__save_cap({
  name: 'fetch_url',
  description: 'Fetch a URL and return byte count — placeholder for size checks.',
  scripts: JSON.stringify([{
    name: 'fetch_url',
    kind: 'tool',
    runtime: 'repl',
    body: handler.toString(),
    schema: JSON.stringify({
      type: 'object',
      properties: {url: {type: 'string'}},
      required: ['url'],
      additionalProperties: false
    })
  }]),
  tags: 'web,fetch,probe',
  tested: true,
  test_date: '2026-05-01',
  auto_active: false
});
```

---

## get_caps

Read cap rows from the DB.

**Optional params**:

| Field | Type | Notes |
|---|---|---|
| `name` | string | Exact match for one cap. |
| `tag` | string | Substring match against `tags`. |
| `project` | string | Project filter. |
| `limit` | int | Default 50. |

**Returns**: array of cap rows. Each row contains the full CapMeta including `Body`, `Scripts`, `Tags`, `Description`, `Version`, `Tested`, `AutoActive`, `Scope`, etc.

**Caveat**: a cap with a >10 KB embedded body returns a >50 KB response. The runtime auto-persists it under `~/.claude/projects/<proj>/<sess>/tool-results/toolu_<id>.json` and only shows a "Output too large" hint inline. Read the persisted file via `Read({file_path, offset, limit})` rather than asking for the inline result.

**Minimal example**:

```javascript
const r = await mcp__yesmem__get_caps({name: 'wiki_export'});
// r is an array; r[0] is the active row
```

---

## activate_cap

Register a cap as a callable tool in the current REPL session.

**Required**:

| Field | Type | Notes |
|---|---|---|
| `name` | string | Cap name. |

**Optional**:

| Field | Type | Notes |
|---|---|---|
| `project` | string | Required for `scope: project` caps. |

**Returns**:

```json
{"code": "registerTool(\"<name>\", \"<desc>\", {<schema>}, async (...) => { ... });"}
```

The `code` field is the registerTool snippet. Eval it in the REPL to bind the cap as a global function. Empty `code` = parse failed; the cap won't be usable. Use this as the **parse-probe self-check** right after `save_cap`.

**Common errors**:

- Empty `code` field → CAP.md doesn't have a parseable `### <tool-kind>` script. Cause: malformed script subsection, missing schema for bash tool, schema on handler, etc.
- `cap not found` → name doesn't match any active learnings row.

**Minimal example**:

```javascript
const r = await mcp__yesmem__activate_cap({name: 'fetch_url'});
if (!r.code) throw new Error('parse failed');
eval(r.code);                            // bind globally in REPL
await fetch_url({url: 'https://example.com'});
```

`registerTool` has two effects: (1) adds entry to internal registry visible via `listTools()`/`getTool()`, (2) binds the handler as a global function in the REPL VM, callable as `<name>({...})`. Tools registered this way do NOT appear in the Claude Code native tool inventory for subsequent turns — only inside REPL blocks.

---

## deactivate_cap

Reverse `activate_cap` for the current thread. Idempotent.

**Required**:

| Field | Type | Notes |
|---|---|---|
| `name` | string | Cap name. |

**Returns**: `{ok: true}` on success.

**Note**: The proxy stops re-injecting the registerTool snippet. The REPL global function may persist for the rest of the current turn but is gone next turn.

---

## register_caps

Generate JS for activating multiple caps at once. Useful at session bootstrap.

**Optional params**:

| Field | Type | Notes |
|---|---|---|
| `project` | string | Filter by project (omit for all). |
| `tag` | string | Filter by tag. |

**Returns**: `{code: "registerTool(...);\nregisterTool(...);\n..."}` — concatenated JS for all matching caps. Eval the whole thing in the REPL.

**Minimal example**:

```javascript
const r = await mcp__yesmem__register_caps({tag: 'fetch'});
eval(r.code);
```

---

## cap_store

The cap-namespaced SQLite RPC. All actions take a `capability` (or `cap` shorthand) param. Tables are auto-prefixed `cap_<capability>__<table>` — pass only the short name.

### `action: 'create_table'`

| Field | Type | Notes |
|---|---|---|
| `capability` | string | Required. |
| `table` | string | Required. Short name. |
| `columns` | string | JSON array of `{name, type}`. Types: `TEXT`, `INTEGER`, `REAL`, `BLOB`. |

`id`, `created_at`, `updated_at` are auto-added — listing them yields `duplicate column name`.

Idempotent: safe to call every handler invocation.

Quotas: 10 tables per cap (`CapsMaxTablesPerCap`).

```javascript
await mcp__yesmem__cap_store({
  capability: 'my_cap', action: 'create_table', table: 'hits',
  columns: JSON.stringify([
    {name:'url', type:'TEXT'},
    {name:'fetched_at', type:'INTEGER'}
  ])
});
```

### `action: 'upsert'`

| Field | Type | Notes |
|---|---|---|
| `capability` | string | Required. |
| `table` | string | Required. Short name. |
| `data` | string | JSON object. Include `id` to UPDATE, omit to INSERT. |

Time-series capture: omit `id` so each call appends with new `created_at`.

```javascript
await mcp__yesmem__cap_store({
  capability: 'my_cap', action: 'upsert', table: 'hits',
  data: JSON.stringify({url:'https://example.com', fetched_at: 1234567890})
});
```

Per-cell limit: 64 KB (`CapsMaxCellBytes`). Larger values yield `cell "X" exceeds 65536 byte limit`.

### `action: 'query'`

| Field | Type | Notes |
|---|---|---|
| `capability` | string | Required. |
| `table` | string | Required. Short name. |
| `where` | string | Optional. SQLite syntax with `?` placeholders. |
| `args` | string | JSON array of bind values for `?` placeholders. |
| `limit` | int | Default 100, max 1000. |
| `offset` | int | Default 0. |

**Returns**: `{rows: [...], count, total, has_more, next_offset}`.

`where` is sanitized: blocked SQL keywords (`UNION|ATTACH|DROP|ALTER|PRAGMA|CREATE|INSERT|UPDATE|DELETE|GRANT|REVOKE|SELECT`) and `;` are rejected after string-literal stripping. `body LIKE '%DELETE%'` is fine; `; DROP TABLE` is not. Response cap ~25 KB per query — for larger reads, page via `offset` or use `cap_search` cap.

```javascript
const r = await mcp__yesmem__cap_store({
  capability: 'my_cap', action: 'query', table: 'hits',
  where: 'fetched_at > ?',
  args: JSON.stringify([Date.now()/1000 - 86400]),
  limit: 50
});
r.rows;  // [{id, url, fetched_at, created_at, updated_at}, ...]
```

### `action: 'delete'`

| Field | Type | Notes |
|---|---|---|
| `capability` | string | Required. |
| `table` | string | Required. Short name. |
| `where` | string | Optional. Omitting deletes all rows. |
| `args` | string | JSON array. |

```javascript
await mcp__yesmem__cap_store({
  capability: 'my_cap', action: 'delete', table: 'hits',
  where: 'url = ?', args: JSON.stringify(['https://example.com'])
});
```

### `action: 'list_tables'`

| Field | Type | Notes |
|---|---|---|
| `capability` | string | Required. |

**Returns**: `["table_a", "table_b", ...]` — short names belonging to the cap.

```javascript
const tables = await mcp__yesmem__cap_store({
  capability: 'my_cap', action: 'list_tables'
});
```

### CLI alternatives

The same RPC is reachable via two CLI surfaces (useful from bash handlers):

```bash
yesmem store '{"capability":"my_cap","action":"query","table":"hits","limit":10}'
yesmem cap-store my_cap query hits "fetched_at > ?" '[1234567890]'
```

Both restricted to `cap_<capname>__<tablename>` tables — there is no generic SQL passthrough through cap_store. For read-only access to yesmem's own learnings tables (outside cap_store), use `yesmem query --format objects` (with single-line jq-shape output) plus `yesmem json` (gojq).

---

## cap_proposal_decide

Accept or reject an auto-correct proposal. Proposals are staged when a `kind: handler` bash script fires under the scheduler, fails, and `auto_correct: true` (default) lets the daemon generate a Sonnet-fixed replacement. The active cap stays untouched until accepted.

**Required**:

| Field | Type | Notes |
|---|---|---|
| `id` | int | learnings.id of the `cap_proposed` row. |
| `decision` | string | `'accept'` or `'reject'`. |

**Optional**:

| Field | Type | Notes |
|---|---|---|
| `notes` | string | Reviewer note appended to the proposal content. |

**Returns**:

```json
{"category": "cap_proposed_accepted" | "cap_proposed_rejected", "id": 12345, ...}
```

On accept: the proposed bash body is applied to the active cap via `save_cap` (with `source: 'auto_correct_accepted'`), proposal transitions to `cap_proposed_accepted`. On reject: proposal transitions to `cap_proposed_rejected`, active cap unchanged.

**Common errors**:

| Error | Cause |
|---|---|
| `'id' is required` | Missing param. |
| `'decision' must be 'accept' or 'reject'` | Bad decision value. |
| `id=N already in state "cap_proposed_accepted"` | Decision already made. |
| `id=N has no cap_proposed:NAME trigger_rule` | Proposal row malformed. |
| `proposal id=N has no bash script "X"` | Cap structure changed since proposal was generated. |

**Caveat (current behavior)**: `autoCorrectBashCap` overwrites the entire cap with one bash body via `save_cap` (`handler_bash` legacy field). On a multi-script bundle this destroys the other scripts. Until phase E lands the script-targeted update, **only single-bash caps benefit from auto-correct**. For multi-script bundles, set `auto_correct: false` on the scheduled job.

**Minimal example**:

```javascript
const list = await mcp__yesmem__list_cap_proposals();
list.forEach(p => console.log(p.id, p.cap_name, p.diff));
await mcp__yesmem__cap_proposal_decide({id: 12345, decision: 'accept'});
```

---

## list_cap_proposals

List auto-correct proposals.

**Optional**:

| Field | Type | Default | Notes |
|---|---|---|---|
| `status` | string | `cap_proposed` | One of: `cap_proposed` (pending), `cap_proposed_accepted`, `cap_proposed_rejected`, `all`. |
| `project` | string | (all) | Project filter. |
| `limit` | int | 100 | Max rows per status. |

**Returns**: array of `{id, cap_name, status, created_at, content (diff/proposal text), original_error, ...}`.

**Minimal example**:

```javascript
const pending = await mcp__yesmem__list_cap_proposals();
const all = await mcp__yesmem__list_cap_proposals({status: 'all', limit: 50});
```

---

## Quick reference: end-to-end build flow

```javascript
// 1. Build + test handler in REPL
const handler = async ({...}) => { /* ... */ };
const test = await handler({...});

// 2. Save (small body — direct call is fine)
await mcp__yesmem__save_cap({
  name: 'my_cap',
  description: 'Discovery-bait one-liner',
  scripts: JSON.stringify([{
    name: 'my_cap', kind: 'tool', runtime: 'repl',
    body: handler.toString(),
    schema: JSON.stringify({type:'object', properties:{...}, additionalProperties:false})
  }]),
  tags: 'web,fetch',
  tested: true,
  test_date: '2026-05-01',
  auto_active: false
});

// 3. Parse-probe — code field non-empty?
const a = await mcp__yesmem__activate_cap({name: 'my_cap'});
if (!a.code) throw new Error('parse-probe failed');
eval(a.code);

// 4. Smoke-call as registered tool
await my_cap({...});

// 5. Verify persistence
await mcp__yesmem__cap_store({capability: 'my_cap', action: 'query', table: 'hits'});

// 6. Confirm via get_caps
await mcp__yesmem__get_caps({name: 'my_cap'});
```

For caps with >10 KB bodies (large embedded scripts, base64 blobs), build the `scripts` JSON in a REPL variable first and never paste the literal as an inline tool argument. See `gotchas.md` "save_cap PLACEHOLDER bug".

---

## yesmem CLI surface for caps

Read-only data access from cap handlers. Use these instead of shelling out to `sqlite3` directly — they enforce the daemon's read-only contract, share the same projection rules as `cap_store`, and are stable.

### `yesmem query` — read-only SQL projection

Runs an arbitrary `SELECT` against one of the four daemon databases. Returns JSON. Default shape is a 2D matrix (`columns` + `rows`); use `--format objects` for an array of `{col: val}` objects (easier to pipe into `yesmem json` or `jq`).

```bash
yesmem query --db yesmem --format objects \
  "SELECT id, category, length(content) AS body_len FROM learnings WHERE category='gotcha' AND superseded_by IS NULL ORDER BY id DESC LIMIT 20"
```

Required flags / params:

| Flag | Values | Default | Purpose |
|------|--------|---------|---------|
| `--db` | `yesmem` · `messages` · `caps` · `runtime` | `yesmem` | Selects which SQLite file under `~/.claude/yesmem/` to open. |
| `--format` | `matrix` · `objects` · `json` | `matrix` | `objects` is the cap-friendly default — every row is `{col: val, …}`. |
| `--limit` | int | none | Hard upper bound on result rows. Tighten in cap handlers — daemon does not enforce a default cap. |

Behavior notes:

- The connection is opened in **read-only** mode. `INSERT`/`UPDATE`/`DELETE`/`PRAGMA writable_schema` raise.
- The query string is passed verbatim to `database/sql` — bind values are not supported. For caps, build the SQL with vetted constants, never with user-controlled fragments.
- The CLI does **not** sanitize column references the way `cap_store.where` does. If a cap exposes user input here, sanitize at the cap layer first.
- For cap-owned data prefer `cap_store(action: 'query')` — it scopes to your cap's tables and enforces the response cap.

Common error:

> `yesmem query: cannot prepare statement: no such table: <name>` — the table lives in a different DB. Re-run with the correct `--db`.

### `yesmem json` — gojq filter pipeline

Pipes JSON through a gojq filter. Same syntax as `jq`, but **no `--slurpfile`** and a few keyword reservations (`label` is reserved — use `lbl`/`name`/`key` for function params).

```bash
yesmem query --db yesmem --format objects \
  "SELECT id, category FROM learnings LIMIT 100" |
  yesmem json '.[] | select(.category == "gotcha") | .id'
```

Replacing `--slurpfile`:

```bash
# instead of: jq --slurpfile aux extra.json '...'
sqlite3 -json yesmem.db "SELECT json_group_array(json_object('k',k,'v',v)) FROM kv" |
  yesmem json 'fromjson | map(.k)'
```

Behavior notes:

- Reads stdin, writes stdout. Suitable for pipes between `yesmem query` and bash-runtime cap handlers.
- gojq is pure-Go (no libjq dependency), but its parser **rejects `label` as a function-parameter name** — see `gotchas.md` "gojq reserved keyword: label".
- Filters that emit non-JSON-compatible values (NaN, undefined identifiers) abort the pipeline; wrap in `try`/`catch` if the data is uncertain.

### `yesmem cap-blob-put` — chunked blob ingest

The escape hatch for the 30 KB `sh()` output wall (Claude Code's stdout limit per tool call). Streams stdin into the daemon and surfaces it as a regular `cap_store` row, bypassing the wall.

```bash
curl -fsSL https://very-large.example/page.html |
  yesmem cap-blob-put --cap mycap --table fetched --col body --key "$(date +%s)"
```

Required flags:

| Flag | Purpose |
|------|---------|
| `--cap` | Owning capability name (must already be registered via `save_cap`). |
| `--table` | `cap_<cap>__<table>` target. |
| `--col` | Column receiving the blob. Must be declared `BLOB` or `TEXT` in the table schema. |
| `--key` | Primary-key value the row is upserted against. |

Behavior notes:

- Default chunk size: 20000 bytes. Smaller chunks = more daemon round-trips; larger chunks risk a `CapsMaxCellBytes` cap on the receiving column.
- Use this **inside** a cap handler that reaches the wall, not as an external workflow tool — it's the bash-runtime equivalent of the JS `await mcp__yesmem__cap_store(...)` you would otherwise write.
- See `recipes.md` "Cap that needs `capblob-pipe`" for an end-to-end pattern.
