---
name: cap_delete
description: "Completely remove a saved capability from both yesmem databases: deletes the learning row + FK-cascade children (entities/actions/keywords/anticipated_queries/cluster_scores) + FTS + session_active_caps + all cap_<name>__* data tables + cap_store_meta entries, wrapped in BEGIN IMMEDIATE transactions on both DBs. Safety-checks knowledge_gaps.resolved_by FK before any write. Input: {cap_name: string} matching [a-zA-Z][a-zA-Z0-9_]*. Irreversible, test target with get_capabilities({name: X}) before calling."
version: 91
tags: [capability, maintenance, destructive]
scope: user
auto_active: true
---

## Purpose

Completely remove a saved capability from both yesmem databases: deletes the learning row + FK-cascade children (entities/actions/keywords/anticipated_queries/cluster_scores) + FTS + session_active_caps + all cap_<name>__* data tables + cap_store_meta entries, wrapped in BEGIN IMMEDIATE transactions on both DBs. Safety-checks knowledge_gaps.resolved_by FK before any write. Input: {cap_name: string} matching [a-zA-Z][a-zA-Z0-9_]*. Irreversible, test target with get_capabilities({name: X}) before calling.

## Scripts

### cap_delete
kind: tool

```javascript
async ({cap_name}) => {
  if (typeof cap_name !== 'string' || !/^[a-zA-Z][a-zA-Z0-9_]*$/.test(cap_name)) {
    return {error: 'cap_name must match [a-zA-Z][a-zA-Z0-9_]*'};
  }
  const ydb = '~/.claude/yesmem/yesmem.db';
  const cdb = '~/.claude/yesmem/capabilities.db';
  const idsRaw = await sh(`sqlite3 ${ydb} "SELECT id FROM learnings WHERE content LIKE '${cap_name} —%';"`);
  const ids = idsRaw.trim().split('\n').filter(s => /^\d+$/.test(s)).map(Number);
  if (!ids.length) return {error: `capability '${cap_name}' not found`};
  const idsSql = `(${ids.join(',')})`;
  const kgRaw = await sh(`sqlite3 ${ydb} "SELECT id,topic FROM knowledge_gaps WHERE resolved_by IN ${idsSql};"`);
  if (kgRaw.trim()) return {error: 'knowledge_gaps FK collision — resolve manually first', refs: kgRaw.trim()};
  const ysql = `BEGIN IMMEDIATE; DELETE FROM learning_entities WHERE learning_id IN ${idsSql}; DELETE FROM learning_actions WHERE learning_id IN ${idsSql}; DELETE FROM learning_keywords WHERE learning_id IN ${idsSql}; DELETE FROM learning_anticipated_queries WHERE learning_id IN ${idsSql}; DELETE FROM learning_cluster_scores WHERE learning_id IN ${idsSql}; DELETE FROM anticipated_queries_fts WHERE learning_id IN ${idsSql}; DELETE FROM session_active_caps WHERE capability_name='${cap_name}'; DELETE FROM learnings_fts WHERE rowid IN ${idsSql}; DELETE FROM learnings WHERE id IN ${idsSql}; COMMIT;`;
  const yerr = await sh(`sqlite3 ${ydb} "${ysql}" 2>&1`);
  if (yerr.trim()) return {error: 'yesmem.db tx failed', detail: yerr};
  const tblsRaw = await sh(`sqlite3 ${cdb} "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'cap_${cap_name}__%';"`);
  const tbls = tblsRaw.trim().split('\n').filter(s => /^cap_[a-zA-Z0-9_]+__[a-zA-Z0-9_]+$/.test(s));
  let dropSql = 'BEGIN IMMEDIATE;';
  for (const t of tbls) dropSql += ` DROP TABLE IF EXISTS ${t};`;
  dropSql += ` DELETE FROM cap_store_meta WHERE cap_name='${cap_name}'; COMMIT;`;
  const cerr = await sh(`sqlite3 ${cdb} "${dropSql}" 2>&1`);
  if (cerr.trim()) return {error: 'capabilities.db tx failed', detail: cerr, partial: {learnings: ids}};
  return {cap_name, deleted: {learnings: ids, data_tables: tbls}};
}
```
